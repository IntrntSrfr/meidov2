package mediaconvertmod

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/intrntsrfr/meido"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"path/filepath"
	"sync"
)

type MediaConvertMod struct {
	sync.Mutex
	name         string
	commands     map[string]*meido.ModCommand
	passives     []*meido.ModPassive
	allowedTypes meido.MessageType
	allowDMs     bool
}

func New(n string) meido.Mod {
	return &MediaConvertMod{
		name:         n,
		commands:     make(map[string]*meido.ModCommand),
		passives:     []*meido.ModPassive{},
		allowedTypes: meido.MessageTypeCreate,
		allowDMs:     true,
	}
}

func (m *MediaConvertMod) Name() string {
	return m.name
}
func (m *MediaConvertMod) Save() error {
	return nil
}
func (m *MediaConvertMod) Load() error {
	return nil
}
func (m *MediaConvertMod) Passives() []*meido.ModPassive {
	return m.passives
}
func (m *MediaConvertMod) Commands() map[string]*meido.ModCommand {
	return m.commands
}
func (m *MediaConvertMod) AllowedTypes() meido.MessageType {
	return m.allowedTypes
}
func (m *MediaConvertMod) AllowDMs() bool {
	return m.allowDMs
}
func (m *MediaConvertMod) Hook(b *meido.Bot) error {

	//m.RegisterCommand(NewMediaConvertCommand(m))
	m.passives = append(m.passives, NewJpgLargeConvertPassive(m))

	return nil
}
func (m *MediaConvertMod) RegisterCommand(cmd *meido.ModCommand) {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.commands[cmd.Name]; ok {
		panic(fmt.Sprintf("command '%v' already exists in %v", cmd.Name, m.Name()))
	}
	m.commands[cmd.Name] = cmd
}

func NewJpgLargeConvertPassive(m *MediaConvertMod) *meido.ModPassive {
	return &meido.ModPassive{
		Mod:          m,
		Name:         "jpglargeconvert",
		Description:  "Automatically converts jpglarge files to jpg",
		AllowedTypes: meido.MessageTypeCreate,
		Enabled:      true,
		Run:          m.jpglargeconvertPassive,
	}
}

func (m *MediaConvertMod) jpglargeconvertPassive(msg *meido.DiscordMessage) {
	if len(msg.Message.Attachments) < 1 {
		return
	}

	var files []*discordgo.File

	for _, att := range msg.Message.Attachments {
		if filepath.Ext(att.URL) != ".jpglarge" {
			continue
		}

		res, err := http.Get(att.URL)
		if err != nil {
			continue
		}

		defer res.Body.Close()

		files = append(files, &discordgo.File{
			Name:   "converted.jpg",
			Reader: res.Body,
		})
	}

	if len(files) < 1 {
		return
	}

	msg.Sess.ChannelMessageSendComplex(msg.Message.ChannelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("%v, I converted that to JPG for you", msg.Author.Mention()),
		Files:   files,
	})
}

func NewMediaConvertCommand(m *MediaConvertMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "mediaconvert",
		Description:   "Converts some media files from one format to another",
		Triggers:      []string{"m?mediaconvert"},
		Usage:         "m?mediaconvert [url] [target format]",
		Cooldown:      30,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      true,
		Enabled:       true,
		Run:           m.mediaconvertCommand,
	}
}

func (m *MediaConvertMod) mediaconvertCommand(msg *meido.DiscordMessage) {
	if msg.LenArgs() < 3 {
		return
	}

	msg.Discord.StartTyping(msg.Message.ChannelID)

	// m?mediaconvert link format

	format := msg.Args()[2]

	if format != "mp4" {
		msg.Reply("invalid format")
		return
	}

	src, err := http.Get(msg.RawArgs()[1])
	if err != nil {
		fmt.Println(err)
		msg.Reply("invalid link")
		return
	}
	defer src.Body.Close()

	fmt.Println(src.StatusCode)
	if src.StatusCode != 200 {
		msg.Reply("invalid link")
		return
	}

	p, _ := ioutil.ReadAll(src.Body)
	if http.DetectContentType(p) != "video/webm" {
		msg.Reply("link is not webm")
		return
	}

	if len(p) > 1024*(1024*6) {
		msg.Reply("file too large")
		return
	}

	inp := &bytes.Buffer{}
	inp.Write(p)

	msg.Reply("this might take a while")

	cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "mp4", "-movflags", "frag_keyframe", "pipe:1")
	//cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "mp4", "pipe:1")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()

	go func() {
		defer stdin.Close()
		io.Copy(stdin, inp)
	}()

	var out bytes.Buffer
	io.Copy(&out, stdout)

	cmd.Wait()

	fmt.Println(out.Len())

	if out.Len() > 1024*(1024*8) {
		msg.Reply("file too large")
	}

	msg.Discord.Sess.ChannelFileSend(msg.Message.ChannelID, "video.mp4", &out)
}
