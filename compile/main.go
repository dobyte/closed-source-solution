package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"sync"

	"github.com/dobyte/closed-source-solution/internal/archive"
	"github.com/dobyte/closed-source-solution/internal/packet"
	"github.com/dobyte/closed-source-solution/internal/utils"
	"github.com/urfave/cli/v3"
)

// 编译命令
// closed-source-compiler --cgo=false --goos=windows --goarch=amd64 --output=main.exe --packages=. --remote=127.0.0.1:8080
func main() {
	cmd := &cli.Command{
		Name:  "closed-source-solution-compile",
		Usage: "local compile client",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "cgo",
				Value: false,
				Usage: "specify whether to enable CGO; it is disabled by default.",
			},
			&cli.StringFlag{
				Name:  "goos",
				Value: runtime.GOOS,
				Usage: "specify the target operating system; default is current operating system.",
			},
			&cli.StringFlag{
				Name:  "goarch",
				Value: runtime.GOARCH,
				Usage: "specify the target architecture; default is current architecture.",
			},
			&cli.StringFlag{
				Name:    "output",
				Value:   "",
				Usage:   "specify the output file name; default is main.exe on Windows, main on other platforms.",
				Aliases: []string{"o"},
			},
			&cli.StringFlag{
				Name:  "packages",
				Value: ".",
				Usage: "specify the path to the package to compile; default is current directory.",
			},
			&cli.StringFlag{
				Name:  "shell",
				Value: "",
				Usage: "specify the shell to execute before compiling; default is empty.",
			},
			&cli.StringFlag{
				Name:     "remote",
				Value:    "",
				Required: true,
				Usage:    "specify the remote server address",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var (
				cgo      = cmd.Bool("cgo")
				goos     = cmd.String("goos")
				goarch   = cmd.String("goarch")
				output   = cmd.String("output")
				packages = cmd.String("packages")
				remote   = cmd.String("remote")
				shell    = cmd.String("shell")
				resp     *packet.ComplieResponse
				file     *os.File
				recv     float64
			)

			if output == "" {
				if goos == "windows" {
					output = "main.exe"
				} else {
					output = "main"
				}
			}

			fmt.Printf("开始连接远端: %s\n", remote)

			conn, err := net.Dial("tcp", remote)
			if err != nil {
				fmt.Printf("连接远端失败: %s\n", errors.Unwrap(errors.Unwrap(err)))
				return nil
			}
			defer conn.Close()

			fmt.Printf("连接远端成功: %s\n", remote)

			fmt.Printf("开始打包源码: %s\n", packages)

			buf, err := archive.Zip(packages)
			if err != nil {
				fmt.Printf("打包源码失败: %s\n", err)
				return nil
			}

			fmt.Printf("打包源码成功: %s\n", utils.BytesToUnits(int64(buf.Len())))

			wg := &sync.WaitGroup{}
			wg.Add(1)

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			go func(conn net.Conn) {
				defer wg.Done()

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
							if resp, err = packet.ParseComplieResponse(data); err != nil {
								fmt.Printf("解析编译失败: %s\n", err)
								return
							}

							if resp.Err != "" {
								fmt.Printf("执行编译失败: %s\n", resp.Err)
								return
							}

							fmt.Printf("执行编译成功: %s %s\n", resp.Exec, utils.BytesToUnits(resp.Size))

							if file, err = os.OpenFile(resp.Exec, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_TRUNC, 0644); err != nil {
								fmt.Printf("创建文件失败: %s\n", err)
								return
							}
							defer file.Close()
						} else {
							n, err := file.Write(data)
							if err != nil {
								fmt.Printf("写入文件失败: %s\n", err)
								return
							}

							recv += float64(n)

							fmt.Printf("正在保存文件: %.2f%s\n", recv/float64(resp.Size)*100, "%")

							if recv >= float64(resp.Size) {
								fmt.Printf("文件保存完成: %s %s\n", resp.Exec, utils.BytesToUnits(resp.Size))
								return
							}
						}
					}
				}
			}(conn)

			if cgo {
				fmt.Printf("发送编译命令: CGO_ENABLED=1 GOOS=%s GOARCH=%s go build -o %s %s\n", goos, goarch, output, packages)
			} else {
				fmt.Printf("发送编译命令: CGO_ENABLED=0 GOOS=%s GOARCH=%s go build -o %s %s\n", goos, goarch, output, packages)
			}

			if err := packet.WriteCompileCommand(conn, &packet.CompileCommand{
				CGO:    cgo,
				GOOS:   goos,
				GOARCH: goarch,
				Output: output,
				Size:   int64(buf.Len()),
				Shell:  shell,
			}); err != nil {
				fmt.Printf("发送命令失败: %s\n", err)
				return nil
			}

			if err := packet.WriteCompileFile(conn, buf.Bytes(), func(i uint8, f float64) {
				fmt.Printf("正在发送文件: %.2f%s\n", f*100, "%")
			}); err != nil {
				fmt.Printf("发送文件失败: %s\n", err)
				return nil
			}

			fmt.Printf("文件发送完成: done\n")

			wg.Wait()

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
