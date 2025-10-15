package utils

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
)

func HTTPGet(url string, headers map[string]string) (*http.Response, error) {
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		DisableCompression:    true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0,
	}

	req, err := http.NewRequest("GET", url, nil)
	if nil != err {
		return nil, err
	}

	req.Header.Set("User-Agent", "Reclaimer/0.1")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return client.Do(req)
}

func HTTPPost(url string, headers map[string]string, body string) (*http.Response, error) {
	client := &http.Client{}

	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if nil != err {
		return nil, err
	}

	req.Header.Set("User-Agent", "Reclaimer/0.1")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return client.Do(req)
}

func MoveFileByPath(sourcePath string, destinationPath string) error {

	err := os.Rename(sourcePath, destinationPath)
	if nil == err {
		return nil
	}

	if (nil != err) && (!errors.Is(err, syscall.EXDEV)) {
		return err
	}

	src, err := os.Open(sourcePath)
	if nil != err {
		return fmt.Errorf("failed to open source when copying to destination: %w", err)
	}

	dest, err := os.Create(destinationPath)
	if nil != err {
		src.Close()
		return fmt.Errorf("failed to open destination for final copy: %w", err)
	}
	defer dest.Close()

	_, err = io.Copy(dest, src)
	if nil != err {
		src.Close()
		return fmt.Errorf("error copying result to final place: %w", err)
	}
	src.Close()
	err = os.Remove(sourcePath)

	return err
}

func MakeOutputPath(sourceName string, outputName string) (string, error) {

	if "" == sourceName {
		return "", fmt.Errorf("expected source name, got empty name")
	}

	cwd, err := os.Getwd()
	if nil != err {
		return "", fmt.Errorf("failed to find current dir: %w", err)
	}

	if "" == outputName {
		outputName = cwd
	} else {
		if !path.IsAbs(outputName) {
			outputName = path.Join(cwd, outputName)
		}
	}

	sourceBaseName := path.Base(sourceName)

	info, err := os.Stat(outputName)
	if nil == err && info.IsDir() {
		return path.Join(outputName, sourceBaseName), nil
	}

	dirPath := path.Dir(outputName)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if nil != err {
		return "", fmt.Errorf("failed to make output dir: %w", err)
	}

	return outputName, nil
}

func DownloadFile(downloadURL string, targetFilename string, extract bool, destinationPath string) error {
	if "" == downloadURL {
		return fmt.Errorf("no download URL provided")
	}
	if "" == targetFilename {
		return fmt.Errorf("download has no name")
	}

	tmpdir, err := os.MkdirTemp("", "reclaimer-*")
	if nil != err {
		return fmt.Errorf("failed to make temp dir: %w", err)
	}
	defer os.RemoveAll(tmpdir)

	tempDownloadPath := path.Join(tmpdir, targetFilename)
	out, err := os.Create(tempDownloadPath)
	if nil != err {
		return fmt.Errorf("failed to create temp download file: %w", err)
	}

	resp, err := HTTPGet(downloadURL, nil)
	if nil != err {
		out.Close()
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		out.Close()
		return fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if nil != err {
		return fmt.Errorf("failed to download file: %w", err)
	}

	if extract {
		zipReader, err := zip.OpenReader(tempDownloadPath)
		if nil != err {
			return fmt.Errorf("failed to open zip file: %w", err)
		}
		defer zipReader.Close()

		generatedFiles := []string{}
		for _, innerFile := range zipReader.File {
			rawDest := path.Join(tmpdir, innerFile.Name)
			dest := path.Clean(rawDest)
			if !strings.HasPrefix(dest, tmpdir) {
				return fmt.Errorf("uncompressing file escapes temp dir: %s", innerFile.Name)
			}

			if innerFile.FileInfo().IsDir() {
				err = os.MkdirAll(dest, os.ModePerm)
				if nil != err {
					return fmt.Errorf("failed to create explicit dir from zip %s: %w", innerFile.Name, err)
				}
			} else {
				dir := path.Dir(dest)
				err = os.MkdirAll(dir, os.ModePerm)
				if nil != err {
					return fmt.Errorf("failed to create implicit dir from zip %s: %w", innerFile.Name, err)
				}

				out, err := os.Create(dest)
				if nil != err {
					return fmt.Errorf("failed to create file for extracted data %s: %w", dest, err)
				}
				defer out.Close()
				compress, err := innerFile.Open()
				if nil != err {
					return fmt.Errorf("failed to open file for extracted data %s: %w", innerFile.Name, err)
				}
				defer compress.Close()

				_, err = io.Copy(out, compress)
				if nil != err {
					return fmt.Errorf("failed to copy data %s: %w", innerFile.Name, err)
				}

				generatedFiles = append(generatedFiles, innerFile.Name)
			}
		}

		// put everything in the final place
		if (1 == len(generatedFiles)) && ("" != destinationPath) {
			destinationPath, err := MakeOutputPath(generatedFiles[0], destinationPath)
			if nil != err {
				return fmt.Errorf("failed to make output path: %w", err)
			}

			err = MoveFileByPath(path.Join(tmpdir, generatedFiles[0]), destinationPath)
			if nil != err {
				return fmt.Errorf("failed to move result to %s: %w", destinationPath, err)
			}
		} else {

			// Treat output as directory
			if "" == destinationPath {
				cwd, err := os.Getwd()
				if nil != err {
					return fmt.Errorf("failed to look up cwd: %w", err)
				}
				destinationPath = cwd
			}

			for _, generatedFileName := range generatedFiles {
				sourcePath := path.Join(tmpdir, generatedFileName)
				finalDestinationPath := path.Join(destinationPath, generatedFileName)
				destinationDirectory := path.Dir(finalDestinationPath)
				err = os.MkdirAll(destinationDirectory, os.ModePerm)
				if nil != err {
					return fmt.Errorf("failed to make output directory %s: %w", destinationDirectory, err)
				}

				err = MoveFileByPath(sourcePath, finalDestinationPath)
				if nil != err {
					return fmt.Errorf("failed to move result to %s: %w", finalDestinationPath, err)
				}
			}
		}

	} else {

		finalDestinationPath, err := MakeOutputPath(targetFilename, destinationPath)
		if nil != err {
			return fmt.Errorf("failed to make output path: %w", err)
		}

		err = MoveFileByPath(tempDownloadPath, finalDestinationPath)
		if nil != err {
			return fmt.Errorf("failed to move result to %s: %w", finalDestinationPath, err)
		}
	}

	return nil
}
