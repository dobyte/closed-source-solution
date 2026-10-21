package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/dobyte/closed-source-solution/internal/archive"
	"github.com/dobyte/closed-source-solution/internal/exec"
	"github.com/dobyte/closed-source-solution/internal/packet"
	"github.com/dobyte/closed-source-solution/internal/utils"
	"github.com/urfave/cli/v3"
)

const (
	// 单个工程压缩包的最大大小
	maxProjectSize = 1 << 30
	// 单个连接两次读之间的最大空闲时间
	readTimeout = 5 * time.Minute
	// 最大并发连接数
	maxConnections = 8
	// 接收缓冲区最大预分配大小
	maxPreallocate = 16 << 20
)

func main() {
	cmd := &cli.Command{
		Name:  "closed-source-solution-remote",
		Usage: "remote compile server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: ":8080",
				Usage: "specify the address of the server",
			},
			&cli.BoolFlag{
				Name:  "tiny",
				Value: false,
				Usage: "specify whether to strip symbols and paths when compiling; it is disabled by default.",
			},
			&cli.BoolFlag{
				Name:  "garble",
				Value: false,
				Usage: "specify whether to enable garble obfuscation when compiling; it is disabled by default.",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cid := 0
			tiny := cmd.Bool("tiny")
			garble := cmd.Bool("garble")

			addr, err := net.ResolveTCPAddr("tcp", cmd.String("addr"))
			if err != nil {
				log.Printf("无效监听地址: %v\n", err)
				return nil
			}

			listener, err := net.ListenTCP(addr.Network(), addr)
			if err != nil {
				log.Printf("启动服务失败: %v\n", err)
				return nil
			}
			defer listener.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sem := make(chan struct{}, maxConnections)

			go func(listener *net.TCPListener) {
				var tempDelay time.Duration

				for {
					conn, err := listener.AcceptTCP()
					if err != nil {
						if errors.Is(err, net.ErrClosed) {
							return
						}

						if tempDelay == 0 {
							tempDelay = 5 * time.Millisecond
						} else {
							tempDelay *= 2
						}
						if max := 1 * time.Second; tempDelay > max {
							tempDelay = max
						}

						log.Printf("接收连接失败: %v\n", err)
						time.Sleep(tempDelay)
						continue
					}

					tempDelay = 0

					select {
					case sem <- struct{}{}:
					default:
						log.Printf("连接数已达上限: %d\n", maxConnections)
						conn.Close()
						continue
					}

					cid++

					go func(cid int, conn net.Conn) {
						defer func() { <-sem }()

						var (
							dir  string
							cmd  *packet.CompileCommand
							buf  *bytes.Buffer
							recv int
						)

						defer func() {
							if err := conn.Close(); err != nil {
								log.Printf("关闭连接失败: cid = %d, %v\n", cid, err)
							} else {
								log.Printf("关闭连接成功: cid = %d\n", cid)
							}

							if dir != "" && utils.IsDir(dir) {
								if err := os.RemoveAll(dir); err != nil {
									log.Printf("删除目录失败: cid = %d, dir = %v, %v\n", cid, dir, err)
								} else {
									log.Printf("删除目录成功: cid = %d, dir = %v\n", cid, dir)
								}
							}
						}()

						for {
							select {
							case <-ctx.Done():
								return
							default:
								if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
									log.Printf("设置读超时失败: cid = %d, %v\n", cid, err)
									return
								}

								header, data, err := packet.ReadMessage(conn)
								if err != nil {
									return
								}

								if header == packet.CompileInfoHeader {
									if cmd, err = packet.ParseComplieCommand(data); err != nil {
										log.Printf("解析命令失败: cid = %d, %v\n", cid, err)
										packet.WriteComplieError(conn, err)
										return
									}

									if cmd.Output == "" || cmd.Output == "." || cmd.Output == ".." || filepath.Base(cmd.Output) != cmd.Output {
										log.Printf("非法的输出文件名: cid = %d, %s\n", cid, cmd.Output)
										packet.WriteComplieError(conn, errors.New("非法的输出文件名"))
										return
									}

									if cmd.Size <= 0 || cmd.Size > maxProjectSize {
										log.Printf("非法的压缩包大小: cid = %d, %d\n", cid, cmd.Size)
										packet.WriteComplieError(conn, errors.New("非法的压缩包大小"))
										return
									}

									dir = strconv.FormatInt(time.Now().UnixNano(), 10)

									tool := "go"
									if garble {
										tool = "garble"
									}

									tagsArg := ""
									if cmd.Tags != "" {
										tagsArg = " -tags=" + cmd.Tags
									}

									if cmd.CGO {
										log.Printf("执行编译命令: cid = %d, CGO_ENABLED=1 GOOS=%s GOARCH=%s %s build%s -o %s %s\n", cid, cmd.GOOS, cmd.GOARCH, tool, tagsArg, cmd.Output, dir)
									} else {
										log.Printf("执行编译命令: cid = %d, CGO_ENABLED=0 GOOS=%s GOARCH=%s %s build%s -o %s %s\n", cid, cmd.GOOS, cmd.GOARCH, tool, tagsArg, cmd.Output, dir)
									}

									buf = &bytes.Buffer{}
									buf.Grow(min(int(cmd.Size), maxPreallocate))
								} else {
									if cmd == nil || buf == nil {
										log.Printf("未收到编译命令: cid = %d\n", cid)
										packet.WriteComplieError(conn, errors.New("未收到编译命令"))
										return
									}

									n, err := buf.Write(data)
									if err != nil {
										log.Printf("写入文件失败: cid = %d, %v\n", cid, err)
										packet.WriteComplieError(conn, err)
										return
									}

									recv += n

									if recv < int(cmd.Size) {
										continue
									}

									log.Printf("文件接收完成: cid = %d, %s\n", cid, utils.BytesToUnits(cmd.Size))

									if err = archive.Unzip(buf, dir); err != nil {
										log.Printf("解压文件失败: cid = %d, %v\n", cid, err)
										packet.WriteComplieError(conn, err)
										return
									}

									log.Printf("解压文件成功: cid = %d, %s\n", cid, dir)

									if cmd.Shell != "" {
										if err = exec.ShellFixed(dir, cmd.Shell); err != nil {
											log.Printf("修复脚本失败: cid = %d, %v\n", cid, err)
											packet.WriteComplieError(conn, err)
											return
										}

										if err = exec.ShellExec(dir, cmd.Shell); err != nil {
											log.Printf("执行命令失败: cid = %d, %v\n", cid, err)
											packet.WriteComplieError(conn, err)
											return
										}
									}

									if err = exec.GoBuild(dir, cmd.CGO, tiny, garble, cmd.GOOS, cmd.GOARCH, cmd.Output, cmd.Tags); err != nil {
										log.Printf("执行编译失败: cid = %d, %v\n", cid, err)
										packet.WriteComplieError(conn, err)
										return
									}

									log.Printf("执行编译成功: cid = %d, %s\n", cid, cmd.Output)

									data, err := os.ReadFile(path.Join(dir, cmd.Output))
									if err != nil {
										log.Printf("读取文件失败: cid = %d, %v\n", cid, err)
										packet.WriteComplieError(conn, err)
										return
									}

									if err := packet.WriteComplieResponse(conn, &packet.ComplieResponse{
										Exec: cmd.Output,
										Size: int64(len(data)),
									}); err != nil {
										log.Printf("发送命令失败: cid = %d, %v\n", cid, err)
										return
									}

									if err := packet.WriteCompileFile(conn, data, func(i uint8, f float64) {
										log.Printf("正在发送文件: cid = %d, %.2f%s\n", cid, f*100, "%")
									}); err != nil {
										log.Printf("发送文件失败: cid = %d, %v\n", cid, err)
									}

									return
								}
							}
						}
					}(cid, conn)
				}
			}(listener)

			log.Printf("启动服务成功: %s\n", cmd.String("addr"))

			sig := make(chan os.Signal, 1)

			switch runtime.GOOS {
			case `windows`:
				signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			default:
				signal.Notify(sig, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)
			}

			<-sig

			signal.Stop(sig)

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
