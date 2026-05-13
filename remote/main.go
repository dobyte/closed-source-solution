package main

import (
	"bytes"
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
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
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cid := 0

			addr, err := net.ResolveTCPAddr("tcp", cmd.String("addr"))
			if err != nil {
				log.Printf("无效监听地址: %s\n", addr)
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

			go func(listener *net.TCPListener) {
				var tempDelay time.Duration

				for {
					conn, err := listener.AcceptTCP()
					if err != nil {
						if e, ok := err.(net.Error); ok && e.Timeout() {
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

						return
					}

					cid++

					go func(cid int, conn net.Conn) {
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

									dir = strconv.FormatInt(time.Now().UnixNano(), 10)

									if cmd.CGO {
										log.Printf("执行编译命令: cid = %d, CGO_ENABLED=1 GOOS=%s GOARCH=%s go build -o %s %s\n", cid, cmd.GOOS, cmd.GOARCH, cmd.Output, dir)
									} else {
										log.Printf("执行编译命令: cid = %d, CGO_ENABLED=0 GOOS=%s GOARCH=%s go build -o %s %s\n", cid, cmd.GOOS, cmd.GOARCH, cmd.Output, dir)
									}

									buf = &bytes.Buffer{}
									buf.Grow(int(cmd.Size))
								} else {
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

									if err = exec.GoBuild(dir, cmd.CGO, cmd.GOOS, cmd.GOARCH, cmd.Output); err != nil {
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

			sig := make(chan os.Signal)

			switch runtime.GOOS {
			case `windows`:
				signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
			default:
				signal.Notify(sig, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGKILL, syscall.SIGTERM)
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
