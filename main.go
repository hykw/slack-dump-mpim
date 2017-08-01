package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/nlopes/slack"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "slack-dump-mpim"
	app.Usage = "Exports only mpim(multiparty direct message) from Slack"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "token, t",
			Value:  "",
			Usage:  "a Slack API token: (see: https://api.slack.com/web)",
			EnvVar: "SLACK_API_TOKEN",
		},
	}
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "Hitoshi Hayakawa",
			Email: "hykw1234@gmail.com",
		},
	}
	app.Version = "1.0.0"
	app.Action = func(c *cli.Context) {
		token := c.String("token")
		if token == "" {
			fmt.Println("ERROR: the token flag is required...")
			fmt.Println("")
			cli.ShowAppHelp(c)
			os.Exit(2)
		}

		pwd, err := os.Getwd()
		check(err)
		outputDir := pwd + "/dump_data"

		// create directory if outputDir does not exists
		if _, err := os.Stat(outputDir); os.IsNotExist(err) {
			os.MkdirAll(outputDir, 0755)
		}

		api := slack.New(token)
		_, err = api.AuthTest()
		if err != nil {
			fmt.Println("ERROR: the token you used is not valid...")
			os.Exit(2)
		}

		// Create working directory
		tmpdir, err := ioutil.TempDir("", "slack-dump")
		check(err)

		dumpMPIM(api, tmpdir)
		archive(tmpdir, outputDir)

		removeTmpDir(tmpdir)
	}

	app.Run(os.Args)
}

func dumpMPIM(api *slack.Client, tmpdir string) {
	groups, err := api.GetGroups(false)
	check(err)

	if len(groups) == 0 {
		return
	}

	for _, group := range groups {
		if strings.HasPrefix(group.Name, "mpdm") {
			dumpChannel(api, tmpdir, group.ID, group.Name, "group")
		}
	}
}

func dumpChannel(api *slack.Client, dir, id, name, channelType string) {
	var messages []slack.Message
	var channelPath string

	if channelType == "group" {
		channelPath = path.Join("private_channel", name)
		messages = fetchGroupHistory(api, id)
	}

	if len(messages) == 0 {
		return
	}

	sort.Sort(byTimestamp(messages))

	currentFilename := ""
	var currentMessages []slack.Message
	for _, message := range messages {
		ts := parseTimestamp(message.Timestamp)
		filename := fmt.Sprintf("%d-%02d-%02d.json", ts.Year(), ts.Month(), ts.Day())
		if currentFilename != filename {
			writeMessagesFile(currentMessages, dir, channelPath, currentFilename)
			currentMessages = make([]slack.Message, 0, 5)
			currentFilename = filename
		}

		currentMessages = append(currentMessages, message)
	}
	writeMessagesFile(currentMessages, dir, channelPath, currentFilename)
}
