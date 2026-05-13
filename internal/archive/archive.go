package archive

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Zip 压缩目录
// 支持压缩go.mod、go.sum、*.go文件
// 其他文件不压缩
func Zip(dir string) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)

	zw := zip.NewWriter(buf)
	defer zw.Close()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		switch {
		case strings.HasSuffix(path, ".go"):
			// add *.go file to zip
		case strings.HasSuffix(path, "go.mod"):
			// add go.mod file to zip
		case strings.HasSuffix(path, "go.sum"):
			// add go.sum file to zip
		case strings.HasSuffix(path, ".sh"):
			// add *.sh file to zip
		default:
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath) // 使用正斜杠
		header.Method = zip.Deflate             // 启用压缩

		if info.IsDir() {
			header.Name += "/"
		}

		writerHeader, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}

		if !info.IsDir() {
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()

			if _, err = io.Copy(writerHeader, in); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// Unzip 解压压缩包到指定目录
func Unzip(buf *bytes.Buffer, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, file := range zr.File {
		if strings.Contains(file.Name, "..") || filepath.IsAbs(file.Name) {
			continue
		}

		filePath := filepath.Join(dir, file.Name)

		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(filePath, 0755); err != nil {
				return err
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return err
		}

		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(filePath)
		if err != nil {
			return err
		}
		defer dst.Close()

		if _, err = io.Copy(dst, src); err != nil {
			return err
		}
	}

	return nil
}
