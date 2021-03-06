package feedbackmod

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/intrntsrfr/meido"
	"sync"
)

type FeedbackMod struct {
	sync.Mutex
	name            string
	commands        map[string]*meido.ModCommand
	bannedUsers     map[string]bool
	feedbackChannel string
	owners          []string
	allowedTypes    meido.MessageType
	allowDMs        bool
}

func New(n string) meido.Mod {
	return &FeedbackMod{
		name:         n,
		commands:     make(map[string]*meido.ModCommand),
		allowedTypes: meido.MessageTypeCreate,
		allowDMs:     true,
	}
}

func (m *FeedbackMod) Name() string {
	return m.name
}
func (m *FeedbackMod) Save() error {
	return nil
}
func (m *FeedbackMod) Load() error {
	return nil
}
func (m *FeedbackMod) Passives() []*meido.ModPassive {
	return []*meido.ModPassive{}
}
func (m *FeedbackMod) Commands() map[string]*meido.ModCommand {
	return m.commands
}
func (m *FeedbackMod) AllowedTypes() meido.MessageType {
	return m.allowedTypes
}
func (m *FeedbackMod) AllowDMs() bool {
	return m.allowDMs
}
func (m *FeedbackMod) Hook(b *meido.Bot) error {
	m.owners = b.Config.OwnerIds

	b.Discord.Sess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Println(r.User.String())
	})

	return nil
}
func (m *FeedbackMod) RegisterCommand(cmd *meido.ModCommand) {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.commands[cmd.Name]; ok {
		panic(fmt.Sprintf("command '%v' already exists in %v", cmd.Name, m.Name()))
	}
	m.commands[cmd.Name] = cmd
}

func (m *FeedbackMod) ToggleBan(msg *meido.DiscordMessage) {
	if msg.LenArgs() <= 1 || msg.Args()[0] != "m?togglefeedback" {
		return
	}

	owner := false
	for _, id := range m.owners {
		if msg.Message.Author.ID == id {
			owner = true
		}
	}
	if !owner {
		return
	}

	m.Lock()
	defer m.Unlock()
	b, ok := m.bannedUsers[msg.Args()[1]]
	if ok {
		if b {
			m.bannedUsers[msg.Args()[1]] = false
			// send unbanned mesage
		} else {
			m.bannedUsers[msg.Args()[1]] = true
			// send banned message
		}
	} else {
		m.bannedUsers[msg.Args()[1]] = true
		// send banned message
	}
}

func (m *FeedbackMod) LeaveFeedback(msg *meido.DiscordMessage) {
	if msg.LenArgs() <= 1 || msg.Args()[0] != "m?feedback" {
		return
	}

	m.Lock()
	defer m.Unlock()
	banned, ok := m.bannedUsers[msg.Message.Author.ID]
	if ok {
		if banned {
			msg.Reply("You're banned from using the feedback feature.")
			return
		}
	}

	msg.Discord.Sess.ChannelMessageSend(m.feedbackChannel, fmt.Sprintf(`%v`, msg.Args()[1:]))
	msg.Reply("Feedback left")
}
