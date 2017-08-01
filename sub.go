package main

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/json"
	"fmt"
	"github.com/nlopes/slack"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func fetchGroupHistory(api *slack.Client, ID string) []slack.Message {
	historyParams := slack.NewHistoryParameters()
	historyParams.Count = 1000

	// Fetch History
	history, err := api.GetGroupHistory(ID, historyParams)
	check(err)
	messages := history.Messages
	latest := messages[len(messages)-1].Timestamp
	for {
		if history.HasMore != true {
			break
		}

		historyParams.Latest = latest
		history, err = api.GetGroupHistory(ID, historyParams)
		check(err)
		length := len(history.Messages)
		if length > 0 {
			latest = history.Messages[length-1].Timestamp
			messages = append(messages, history.Messages...)
		}

	}

	return messages
}

func parseTimestamp(timestamp string) *time.Time {
	if utf8.RuneCountInString(timestamp) <= 0 {
		return nil
	}

	ts := timestamp

	if strings.Contains(timestamp, ".") {
		e := strings.Split(timestamp, ".")
		if len(e) != 2 {
			return nil
		}
		ts = e[0]
	}

	i, err := strconv.ParseInt(ts, 10, 64)
	check(err)
	tm := time.Unix(i, 0).Local()
	return &tm
}

func writeMessagesFile(messages []slack.Message, dir string, channelPath string, filename string) {
	if len(messages) == 0 || dir == "" || channelPath == "" || filename == "" {
		return
	}
	channelDir := path.Join(dir, channelPath)
	err := os.MkdirAll(channelDir, 0755)
	check(err)

	data, err := MarshalIndent(messages, "", "    ")
	check(err)
	err = ioutil.WriteFile(path.Join(channelDir, filename), data, 0644)
	check(err)
}

// MarshalIndent is like json.MarshalIndent but applies Slack's weird JSON
// escaping rules to the output.
func MarshalIndent(v interface{}, prefix string, indent string) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return nil, err
	}

	b = bytes.Replace(b, []byte("\\u003c"), []byte("<"), -1)
	b = bytes.Replace(b, []byte("\\u003e"), []byte(">"), -1)
	b = bytes.Replace(b, []byte("\\u0026"), []byte("&"), -1)
	b = bytes.Replace(b, []byte("/"), []byte("\\/"), -1)

	return b, nil
}

func archive(inFilePath, outputDir string) {
	ts := time.Now().Format("20060102150405")
	outZipPath := path.Join(outputDir, fmt.Sprintf("slackdump-%s.zip", ts))

	outZip, err := os.Create(outZipPath)
	check(err)
	defer outZip.Close()

	zipWriter := zip.NewWriter(outZip)
	defer zipWriter.Close()

	// Set compression level: flate.BestCompression
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	basePath := filepath.Dir(inFilePath)

	err = filepath.Walk(inFilePath, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil || fileInfo.IsDir() {
			return err
		}

		relativeFilePath, err := filepath.Rel(basePath, filePath)
		if err != nil {
			return err
		}

		// do not include ioutil.TempDir name
		relativeFilePathArr := strings.Split(relativeFilePath, string(filepath.Separator))
		relativeFilePath = path.Join(relativeFilePathArr[1:]...)

		archivePath := path.Join(filepath.SplitList(relativeFilePath)...)

		fmt.Println(archivePath)

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		zipFileWriter, err := zipWriter.Create(archivePath)
		if err != nil {
			return err
		}

		_, err = io.Copy(zipFileWriter, file)
		return err
	})

	check(err)
}

func removeTmpDir(tmpdir string) {
	if err := os.RemoveAll(tmpdir); err != nil {
		fmt.Println(err)
	}
}
