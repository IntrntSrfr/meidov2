package utilitymod

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/intrntsrfr/meido"
	"github.com/jmoiron/sqlx"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type UtilityMod struct {
	sync.Mutex
	name         string
	commands     map[string]*meido.ModCommand
	startTime    time.Time
	db           *sqlx.DB
	allowedTypes meido.MessageType
	allowDMs     bool
}

func New(name string) meido.Mod {
	return &UtilityMod{
		startTime:    time.Now(),
		name:         name,
		commands:     make(map[string]*meido.ModCommand),
		allowedTypes: meido.MessageTypeCreate,
		allowDMs:     true,
	}
}

func (m *UtilityMod) Name() string {
	return m.name
}
func (m *UtilityMod) Save() error {
	return nil
}
func (m *UtilityMod) Load() error {
	return nil
}
func (m *UtilityMod) Passives() []*meido.ModPassive {
	return []*meido.ModPassive{}
}
func (m *UtilityMod) Commands() map[string]*meido.ModCommand {
	return m.commands
}
func (m *UtilityMod) AllowedTypes() meido.MessageType {
	return m.allowedTypes
}
func (m *UtilityMod) AllowDMs() bool {
	return m.allowDMs
}
func (m *UtilityMod) Hook(b *meido.Bot) error {
	//m.cl = b.CommandLog
	m.db = b.DB

	statusTimer := time.NewTicker(time.Second * 15)
	b.Discord.Sess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		oldMemCount := 0
		oldSrvCount := 0
		display := true
		go func() {
			for range statusTimer.C {
				if display {
					memCount := 0
					srvCount := 0
					for _, g := range b.Discord.Guilds() {
						srvCount++
						memCount += g.MemberCount
					}
					/*
						if memCount == oldMemCount && srvCount == oldSrvCount {
							continue
						}
					*/
					s.UpdateStatusComplex(discordgo.UpdateStatusData{
						Game: &discordgo.Game{
							Name: fmt.Sprintf("over %v servers and %v members", srvCount, memCount),
							Type: discordgo.GameTypeWatching,
						},
					})
					oldMemCount = memCount
					oldSrvCount = srvCount
				} else {
					s.UpdateStatusComplex(discordgo.UpdateStatusData{
						Game: &discordgo.Game{
							Name: fmt.Sprintf("m?help"),
							Type: discordgo.GameTypeGame,
						},
					})
				}
				display = !display
			}
		}()
	})

	m.RegisterCommand(NewAvatarCommand(m))
	m.RegisterCommand(NewAboutCommand(m))
	m.RegisterCommand(NewServerCommand(m))
	m.RegisterCommand(NewServerAvatarCommand(m))
	m.RegisterCommand(NewServerBannerCommand(m))
	m.RegisterCommand(NewServerSplashCommand(m))
	m.RegisterCommand(NewColorCommand(m))
	m.RegisterCommand(NewInviteCommand(m))
	//m.RegisterCommand(NewUserPermsCommand(m))
	m.RegisterCommand(NewUserInfoCommand(m))

	return nil
}
func (m *UtilityMod) RegisterCommand(cmd *meido.ModCommand) {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.commands[cmd.Name]; ok {
		panic(fmt.Sprintf("command '%v' already exists in %v", cmd.Name, m.name))
	}
	m.commands[cmd.Name] = cmd
}

func NewAvatarCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "avatar",
		Description:   "Displays profile picture of user or mentioned user",
		Triggers:      []string{"m?avatar", "m?av", ">av"},
		Usage:         ">av | >av 123123123123",
		Cooldown:      2,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      true,
		Enabled:       true,
		Run:           m.avatarCommand,
	}
}

func (m *UtilityMod) avatarCommand(msg *meido.DiscordMessage) {
	if msg.LenArgs() < 1 {
		return
	}

	var targetUser *discordgo.User
	var err error

	if msg.LenArgs() > 1 {
		if len(msg.Message.Mentions) >= 1 {
			targetUser = msg.Message.Mentions[0]
		} else {
			if _, err = strconv.Atoi(msg.Args()[1]); err != nil {
				return
			}
			tm, err := msg.Discord.Member(msg.Message.GuildID, msg.Args()[1])
			if err != nil {
				targetUser, err = msg.Sess.User(msg.Args()[1])
				if err != nil {
					return
				}
			} else {
				targetUser = tm.User
			}
		}
	} else {
		targetUser = msg.Message.Author
	}

	if targetUser == nil {
		return
	}

	if targetUser.Avatar == "" {
		msg.ReplyEmbed(&discordgo.MessageEmbed{
			Color:       0xC80000,
			Description: fmt.Sprintf("%v has no avatar set.", targetUser.String()),
		})
	} else {
		msg.ReplyEmbed(&discordgo.MessageEmbed{
			Color: msg.Discord.HighestColor(msg.Message.GuildID, targetUser.ID),
			Title: targetUser.String(),
			Image: &discordgo.MessageEmbedImage{URL: targetUser.AvatarURL("1024")},
		})
	}
}

func NewServerCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "server",
		Description:   "Displays information about the server",
		Triggers:      []string{"m?server"},
		Usage:         "m?server",
		Cooldown:      10,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      false,
		Enabled:       true,
		Run:           m.serverCommand,
	}
}

func (m *UtilityMod) serverCommand(msg *meido.DiscordMessage) {
	if msg.LenArgs() < 1 {
		return
	}

	g, err := msg.Discord.Guild(msg.Message.GuildID)
	if err != nil {
		msg.Reply("Error getting guild data")
		return
	}

	tc := 0
	vc := 0

	for _, ch := range g.Channels {
		if ch.Type == discordgo.ChannelTypeGuildText {
			tc++
		} else if ch.Type == discordgo.ChannelTypeGuildVoice {
			vc++
		}
	}

	users := 0
	bots := 0

	for _, m := range g.Members {
		if m.User.Bot {
			bots++
		} else {
			users++
		}
	}

	owner, err := msg.Discord.Member(g.ID, g.OwnerID)
	if err != nil {
		msg.Reply("Error getting guild data")
		return
	}

	ts := meido.IDToTimestamp(g.ID)
	dur := time.Since(ts)

	embed := discordgo.MessageEmbed{
		Color: 0xFEFEFE,
		Author: &discordgo.MessageEmbedAuthor{
			Name: g.Name,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Owner",
				Value:  fmt.Sprintf("%v\n(%v)", owner.Mention(), owner.User.ID),
				Inline: true,
			},
			{
				Name:  "Creation date",
				Value: fmt.Sprintf("%v | %v day(s) ago", ts.Format(time.RFC1123), math.Floor(dur.Hours()/24.0)),
			},
			{
				Name:   "Members",
				Value:  fmt.Sprintf("%v members\n%v users\n%v bots", g.MemberCount, users, bots),
				Inline: true,
			},
			{
				Name:   "Channels",
				Value:  fmt.Sprintf("Total: %v\nText: %v\nVoice: %v", len(g.Channels), tc, vc),
				Inline: true,
			},
			{
				Name:   "Roles",
				Value:  fmt.Sprintf("%v roles", len(g.Roles)),
				Inline: true,
			},
		},
	}
	if g.Icon != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: fmt.Sprintf("https://cdn.discordapp.com/icons/%v/%v.png", g.ID, g.Icon),
		}
		embed.Author.IconURL = fmt.Sprintf("https://cdn.discordapp.com/icons/%v/%v.png", g.ID, g.Icon)
	}

	msg.ReplyEmbed(&embed)
}

func NewAboutCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "about",
		Description:   "Displays Meido statistics",
		Triggers:      []string{"m?about"},
		Usage:         "m?about",
		Cooldown:      10,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowDMs:      true,
		AllowedTypes:  meido.MessageTypeCreate,
		Enabled:       true,
		Run:           m.aboutCommand,
	}
}

func (m *UtilityMod) aboutCommand(msg *meido.DiscordMessage) {
	if msg.LenArgs() < 1 {
		return
	}

	var (
		totalUsers int

		totalBots   int
		totalHumans int

		memory        runtime.MemStats
		totalCommands int
	)
	runtime.ReadMemStats(&memory)
	guilds := msg.Discord.Guilds()

	for _, guild := range guilds {
		for _, mem := range guild.Members {
			if mem.User.Bot {
				totalBots++
			} else {
				totalHumans++
			}
		}

		totalUsers += guild.MemberCount
	}

	uptime := time.Now().Sub(m.startTime)
	err := m.db.Get(&totalCommands, "SELECT COUNT(*) FROM commandlog;")
	if err != nil {
		msg.Reply("Error getting data")
		return
	}

	msg.ReplyEmbed(&discordgo.MessageEmbed{
		Title: "About",
		Color: 0xFEFEFE,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Uptime",
				Value:  uptime.String(),
				Inline: true,
			},
			{
				Name:   "Total commands ran",
				Value:  strconv.Itoa(totalCommands),
				Inline: true,
			},
			{
				Name:   "Guilds",
				Value:  strconv.Itoa(len(guilds)),
				Inline: false,
			},
			{
				Name:   "Users",
				Value:  fmt.Sprintf("%v users | %v humans | %v bots", totalUsers, totalHumans, totalBots),
				Inline: true,
			},
			{
				Name:   "Current memory use",
				Value:  fmt.Sprintf("%v/%v", humanize.Bytes(memory.Alloc), humanize.Bytes(memory.Sys)),
				Inline: false,
			},
			{
				Name:   "Garbage collected",
				Value:  humanize.Bytes(memory.TotalAlloc - memory.Alloc),
				Inline: true,
			},
			/*
				{
					Name:   "Allocs | Frees",
					Value:  fmt.Sprintf("%v | %v", memory.Mallocs, memory.Frees),
					Inline: false,
				},
			*/
		},
	})
}

func NewServerSplashCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "serversplash",
		Description:   "Displays server splash if one exists",
		Triggers:      []string{"m?serversplash"},
		Usage:         "m?serversplash",
		Cooldown:      5,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      false,
		Enabled:       true,
		Run:           m.serverSplashCommand,
	}
}
func (m *UtilityMod) serverSplashCommand(msg *meido.DiscordMessage) {
	if msg.LenArgs() < 1 {
		return
	}

	g, err := msg.Discord.Guild(msg.Message.GuildID)
	if err != nil {
		return
	}

	if g.Splash == "" {
		msg.Reply("this server has no splash")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: g.Name,
		Color: 0xFFFFFF,
		Image: &discordgo.MessageEmbedImage{
			URL: fmt.Sprintf("https://cdn.discordapp.com/splashes/%v/%v.png?size=2048", g.ID, g.Splash),
		},
	}
	msg.ReplyEmbed(embed)
}
func NewServerAvatarCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "serveravatar",
		Description:   "Displays server avatar if one exists",
		Triggers:      []string{"m?serveravatar", "m?servericon"},
		Usage:         "m?servericon",
		Cooldown:      5,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      false,
		Enabled:       true,
		Run:           m.serverIconCommand,
	}
}
func (m *UtilityMod) serverIconCommand(msg *meido.DiscordMessage) {
	if msg.LenArgs() < 1 {
		return
	}

	g, err := msg.Discord.Guild(msg.Message.GuildID)
	if err != nil {
		return
	}

	if g.Icon == "" {
		msg.Reply("this server has no avatar")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: g.Name,
		Color: 0xFFFFFF,
		Image: &discordgo.MessageEmbedImage{
			URL: fmt.Sprintf("%v?size=2048", g.IconURL()),
		},
	}
	msg.ReplyEmbed(embed)
}

func NewServerBannerCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "serverbanner",
		Description:   "Displays server banner if one exists",
		Triggers:      []string{"m?serverbanner"},
		Usage:         "m?serverbanner",
		Cooldown:      5,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      false,
		Enabled:       true,
		Run:           m.serverBannerCommand,
	}
}
func (m *UtilityMod) serverBannerCommand(msg *meido.DiscordMessage) {
	if msg.LenArgs() < 1 {
		return
	}

	g, err := msg.Discord.Guild(msg.Message.GuildID)
	if err != nil {
		return
	}

	if g.Splash == "" {
		msg.Reply("this server has no banner")
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: g.Name,
		Color: 0xFFFFFF,
		Image: &discordgo.MessageEmbedImage{
			URL: fmt.Sprintf("https://cdn.discordapp.com/banners/%v/%v.png?size=2048", g.ID, g.Banner),
		},
	}
	msg.ReplyEmbed(embed)
}

func NewColorCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "color",
		Description:   "Displays a hex color",
		Triggers:      []string{"m?color"},
		Usage:         "m?color #c0ffee\nm?color c0ffee",
		Cooldown:      1,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      true,
		Enabled:       true,
		Run:           m.colorCommand,
	}
}
func (m *UtilityMod) colorCommand(msg *meido.DiscordMessage) {
	if msg.LenArgs() < 2 {
		return
	}

	clrStr := msg.Args()[1]

	if clrStr[0] == '#' {
		clrStr = clrStr[1:]
	}

	clr, err := strconv.ParseInt(clrStr, 16, 32)
	if err != nil {
		msg.Reply("invalid color")
		return
	}
	if clr < 0 || clr > 0xffffff {
		msg.Reply("invalid color")
		return
	}

	red := clr >> 16
	green := (clr >> 8) & 0xff
	blue := clr & 0xff

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))

	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: uint8(red), G: uint8(green), B: uint8(blue), A: 255}}, image.Point{}, draw.Src)

	buf := &bytes.Buffer{}

	err = png.Encode(buf, img)
	if err != nil {
		return
	}

	msg.Sess.ChannelFileSend(msg.Message.ChannelID, "color.png", buf)
}

func NewInviteCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "invite",
		Description:   "Sends an invite link for Meido, as well as support server",
		Triggers:      []string{"m?invite"},
		Usage:         "m?invite",
		Cooldown:      1,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      true,
		Enabled:       true,
		Run:           m.inviteCommand,
	}
}
func (m *UtilityMod) inviteCommand(msg *meido.DiscordMessage) {
	botLink := "<https://discordapp.com/oauth2/authorize?client_id=394162399348785152&scope=bot>"
	serverLink := "https://discord.gg/KgMEGK3"
	msg.Reply(fmt.Sprintf("Invite me to your server: %v\nSupport server: %v", botLink, serverLink))
}

func NewUserPermsCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "userperms",
		Description:   "Displays what permissions a user has in the current channel",
		Triggers:      []string{"m?userperms"},
		Usage:         "m?userperms | m?userperms @user",
		Cooldown:      2,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      false,
		Enabled:       true,
		Run:           m.userpermsCommand,
	}
}

func (m *UtilityMod) userpermsCommand(msg *meido.DiscordMessage) {

	var (
		err        error
		targetUser *discordgo.Member
	)

	if msg.LenArgs() > 1 {
		if len(msg.Message.Mentions) > 0 {
			targetUser, err = msg.Discord.Member(msg.Message.GuildID, msg.Message.Mentions[0].ID)
			if err != nil {
				return
			}
		} else {
			if _, err := strconv.Atoi(msg.Args()[1]); err != nil {
				return
			}
			targetUser, err = msg.Discord.Member(msg.Message.GuildID, msg.Args()[1])
			if err != nil {
				return
			}
		}
	} else {
		targetUser = msg.Member
	}

	uPerms, err := msg.Discord.UserChannelPermissionsDirect(targetUser, msg.Message.ChannelID)
	if err != nil {
		return
	}

	sb := strings.Builder{}
	sb.WriteString("```\n")
	sb.WriteString(fmt.Sprintf("perm binary: %032b\n\n", uPerms))
	for k, v := range meido.PermMap {
		if k == 0 {
			continue
		}

		if uPerms&k != 0 {
			sb.WriteString(fmt.Sprintf("%v - true\n", v))
		} else {
			sb.WriteString(fmt.Sprintf("%v - false\n", v))
		}
	}
	sb.WriteString("```")

	msg.Reply(sb.String())
}

func NewUserInfoCommand(m *UtilityMod) *meido.ModCommand {
	return &meido.ModCommand{
		Mod:           m,
		Name:          "userinfo",
		Description:   "Displays information about a user",
		Triggers:      []string{"m?userinfo"},
		Usage:         "m?userinfo | m?userinfo @user",
		Cooldown:      1,
		RequiredPerms: 0,
		RequiresOwner: false,
		AllowedTypes:  meido.MessageTypeCreate,
		AllowDMs:      false,
		Enabled:       true,
		Run:           m.userinfoCommand,
	}
}
func (m *UtilityMod) userinfoCommand(msg *meido.DiscordMessage) {

	var (
		targetUser   *discordgo.User
		targetMember *discordgo.Member
	)

	if msg.LenArgs() > 1 {
		if len(msg.Message.Mentions) >= 1 {
			targetUser = msg.Message.Mentions[0]
			targetMember, _ = msg.Discord.Member(msg.Message.GuildID, msg.Message.Mentions[0].ID)
		} else {
			_, err := strconv.Atoi(msg.Args()[1])
			if err != nil {
				return
			}
			targetMember, err = msg.Discord.Member(msg.Message.GuildID, msg.Args()[1])
			if err != nil {
				targetUser, err = msg.Sess.User(msg.Args()[1])
				if err != nil {
					return
				}
			} else {
				targetUser = targetMember.User
			}
		}
	} else {
		targetMember = msg.Member
		targetUser = targetMember.User
	}

	createTs := meido.IDToTimestamp(targetUser.ID)
	createDur := time.Since(createTs)

	emb := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("User info | %v", targetUser.String()),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: targetUser.AvatarURL("512"),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "ID | Mention",
				Value:  fmt.Sprintf("%v | <@!%v>", targetUser.ID, targetUser.ID),
				Inline: false,
			},
			{
				Name:   "Creation date",
				Value:  fmt.Sprintf("%v | %v day(s) ago", createTs.Format(time.RFC1123), math.Floor(createDur.Hours()/24.0)),
				Inline: false,
			},
		},
	}

	if targetMember != nil {

		joinTs, err := targetMember.JoinedAt.Parse()
		if err != nil {
			msg.Reply("something terrible happened")
			return
		}
		joinDur := time.Since(joinTs)

		nick := targetMember.Nick
		if nick == "" {
			nick = "None"
		}

		emb.Color = msg.Discord.HighestColor(msg.Message.GuildID, targetMember.User.ID)
		emb.Fields = append(emb.Fields, &discordgo.MessageEmbedField{
			Name:   "Join date",
			Value:  fmt.Sprintf("%v | %v day(s) ago", joinTs.Format(time.RFC1123), math.Floor(joinDur.Hours()/24.0)),
			Inline: false,
		})
		emb.Fields = append(emb.Fields, &discordgo.MessageEmbedField{
			Name:   "Roles",
			Value:  strconv.Itoa(len(targetMember.Roles)),
			Inline: true,
		})
		emb.Fields = append(emb.Fields, &discordgo.MessageEmbedField{
			Name:   "Nickname",
			Value:  nick,
			Inline: true,
		})

	}
	msg.ReplyEmbed(emb)
}
