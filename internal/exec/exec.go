package exec

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// GoEnv 解析go env命令的输出
// 返回一个字符串数组，每个元素是一个环境变量，格式为"key=value"
// 如果解析失败，返回错误
func GoEnv() ([]string, error) {
	cmd := exec.Command("go", "env")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	rows := regexp.MustCompile(`[A-Z]+=\'.*\'`).FindAllString(string(output), -1)
	envs := make([]string, 0, len(rows))

	for _, row := range rows {
		parts := strings.SplitN(row, "=", 2)
		if len(parts) != 2 {
			continue
		}

		envs = append(envs, parts[0]+"="+strings.Trim(parts[1], "'"))
	}

	return envs, nil
}

// GoBuild 编译go项目
// tiny 为 true 时附加精简参数（-trimpath -buildvcs=false -ldflags="-s -w -buildid="）
// garble 为 true 时使用 garble 进行代码混淆编译
// tags 非空时通过 -tags 指定构建标签
// 返回错误
// 如果编译成功，返回 nil
func GoBuild(dir string, cgo bool, tiny bool, garble bool, goos string, goarch string, output string, tags string) error {
	stderr := &bytes.Buffer{}

	name, args := "go", []string{"build"}
	if garble {
		name, args = "garble", []string{"-literals", "-tiny", "-seed=random", "build"}
	}

	if tiny {
		args = append(args, "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=")
	}

	if tags != "" {
		args = append(args, "-tags="+tags)
	}

	args = append(args, "-o", output, ".")

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOOS="+goos)
	cmd.Env = append(cmd.Env, "GOARCH="+goarch)
	cmd.Stderr = stderr

	if cgo {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	} else {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}

	if err := cmd.Run(); err != nil {
		if msg := stderr.String(); msg != "" {
			return errors.New(msg)
		}

		return err
	}

	return nil
}

// GoGet 获取go模块
// 返回错误
// 如果获取成功，返回 nil
func GoGet(envs []string, dir string, mod string) error {
	stderr := &bytes.Buffer{}

	cmd := exec.Command("go", "get", mod)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return errors.New(stderr.String())
	}

	return nil
}

// GoModTidy 修复go mod tidy命令
// 返回错误
// 如果修复成功，返回 nil
func GoModTidy(envs []string, dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = envs

	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}

	return nil
}

// ShellFixed 修复shell脚本中的换行符
// 返回错误
// 如果修复成功，返回 nil
func ShellFixed(dir string, shell string) error {
	stderr := &bytes.Buffer{}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("sed -i 's/\r$//' %s && chmod +x %s", shell, shell))
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if msg := stderr.String(); msg != "" {
			return errors.New(msg)
		}

		return err
	}

	return nil
}

// Shell 执行shell命令
// 返回错误
// 如果执行成功，返回 nil
func ShellExec(dir string, shell string) error {
	stderr := &bytes.Buffer{}

	cmd := exec.Command("sh", "-c", "./"+shell)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if msg := stderr.String(); msg != "" {
			return errors.New(msg)
		}

		return err
	}

	return nil
}
