package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Command line flags
var (
	BotToken = flag.String("token", "MTE2MTY1NTMzMjQ5NDM4OTI5OQ.Go4XZI.E8PsXWAhapTsloEjdFVyCJvteYJdMAdMvZf25Q", "Bot authorization token")
	appID    = flag.String("app", "1161655332494389299", "Application ID")
	// guildID  = flag.String("guild", "778318814755815516", "ID of the testing guild")
	guildID = flag.String("guild", "342812061207887872", "ID of the testing guild")
	// ChannelID = flag.String("channel", "1161658197019463791", "ID of the testing channel")
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutdowning or not")
)

var s *discordgo.Session

func init() { flag.Parse() }

func init() {
	var err error
	s, err = discordgo.New("Bot " + *BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
}

var (
	dmPermission                   = false
	integerOptionMinValue          = 1.0
	defaultMemberPermissions int64 = discordgo.PermissionManageServer
	shadowbannedUserIDs            = make(map[string]*discordgo.User)
	commands                       = []*discordgo.ApplicationCommand{
		{
			Name:        "hello-world",
			Description: "Showcase of a basic slash command",
		},
		{
			Name:                     "permission-overview",
			Description:              "Command for demonstration of default command permissions",
			DefaultMemberPermissions: &defaultMemberPermissions,
			DMPermission:             &dmPermission,
		},
		{
			Name:                     "delete-messages",
			Description:              "Command for deleting specified messages in this channel, limitied to 100 messages",
			DefaultMemberPermissions: &defaultMemberPermissions,
			DMPermission:             &dmPermission,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "number",
					Description: "Number of messages, limited to 100 messages",
					MinValue:    &integerOptionMinValue,
					MaxValue:    100,
					Required:    true,
				},
			},
		},
		{
			Name:                     "shadow-ban",
			Description:              "Delete every message the shadowbanned user sends",
			DefaultMemberPermissions: &defaultMemberPermissions,
			DMPermission:             &dmPermission,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "User option",
					Required:    true,
				},
			},
		},
		{
			Name:                     "shadow-unban",
			Description:              "Remove user from shadowban list",
			DefaultMemberPermissions: &defaultMemberPermissions,
			DMPermission:             &dmPermission,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "User option",
					Required:    true,
				},
			},
		},
		{
			Name:                     "shadow-ban-list",
			Description:              "Shows list of shadowban users",
			DefaultMemberPermissions: &defaultMemberPermissions,
			DMPermission:             &dmPermission,
		},
	}
	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"hello-world": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Hello world!",
				},
			})
		},
		"permission-overview": func(s *discordgo.Session, i *discordgo.InteractionCreate) {

			perms, err := s.ApplicationCommandPermissions(s.State.User.ID, i.GuildID, i.ApplicationCommandData().ID)

			var restError *discordgo.RESTError
			if errors.As(err, &restError) && restError.Message != nil && restError.Message.Code == discordgo.ErrCodeUnknownApplicationCommandPermissions {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: ":x: No permission overwrites",
					},
				})
				return
			} else if err != nil {
				panic(err)
			}

			if err != nil {
				panic(err)
			}
			format := "- %s %s\n"

			channels := ""
			users := ""
			roles := ""

			for _, o := range perms.Permissions {
				emoji := "❌"
				if o.Permission {
					emoji = "☑"
				}

				switch o.Type {
				case discordgo.ApplicationCommandPermissionTypeUser:
					users += fmt.Sprintf(format, emoji, "<@!"+o.ID+">")
				case discordgo.ApplicationCommandPermissionTypeChannel:
					allChannels, _ := discordgo.GuildAllChannelsID(i.GuildID)

					if o.ID == allChannels {
						channels += fmt.Sprintf(format, emoji, "All channels")
					} else {
						channels += fmt.Sprintf(format, emoji, "<#"+o.ID+">")
					}
				case discordgo.ApplicationCommandPermissionTypeRole:
					if o.ID == i.GuildID {
						roles += fmt.Sprintf(format, emoji, "@everyone")
					} else {
						roles += fmt.Sprintf(format, emoji, "<@&"+o.ID+">")
					}
				}
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "Permissions overview",
							Description: "Overview of permissions for this command",
							Fields: []*discordgo.MessageEmbedField{
								{
									Name:  "Users",
									Value: users,
								},
								{
									Name:  "Channels",
									Value: channels,
								},
								{
									Name:  "Roles",
									Value: roles,
								},
							},
						},
					},
					AllowedMentions: &discordgo.MessageAllowedMentions{},
				},
			})
		},

		"delete-messages": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			var limit int = int(options[0].IntValue())
			c, err := s.State.Channel(i.ChannelID)
			if err != nil {
				return
			}

			messages, err := s.ChannelMessages(c.ID, limit, "", "", "")
			if err != nil {
				return
			}

			var messageIDs []string
			for _, msg := range messages {
				messageIDs = append(messageIDs, msg.ID)
			}
			err = s.ChannelMessagesBulkDelete(c.ID, messageIDs)
			if err != nil {
				fmt.Println(err)
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: "Deleting messages, this message will be deleted after 5 seconds!",
				},
			})

			time.Sleep(time.Second * 5)
			s.InteractionResponseDelete(i.Interaction)
		},
		"shadow-ban": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			shadowbannedUserIDs[options[0].UserValue(nil).ID] = options[0].UserValue(s)
			format := "User %s has been shadowbanned!\n"
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:         fmt.Sprintf(format, "<@!"+options[0].UserValue(nil).ID+">"),
					AllowedMentions: &discordgo.MessageAllowedMentions{},
				},
			})
		},
		"shadow-unban": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			delete(shadowbannedUserIDs, options[0].UserValue(nil).ID)
			format := "User %s has been unbanned!\n"
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:         fmt.Sprintf(format, "<@!"+options[0].UserValue(nil).ID+">"),
					AllowedMentions: &discordgo.MessageAllowedMentions{},
				},
			})
		},
		"shadow-ban-list": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			users := ""
			format := "- %s\n"
			for _, user := range shadowbannedUserIDs {
				users += fmt.Sprintf(format, "<@!"+user.ID+">")
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title: "Shadowbanned users:",
							// Description: "Overview of permissions for this command",
							Fields: []*discordgo.MessageEmbedField{
								{
									// Name:  "Shadowbanned users:",
									Value: users,
								},
							},
						},
					},
					AllowedMentions: &discordgo.MessageAllowedMentions{},
				},
			})
		},
	}
)

func init() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func main() {
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})

	// sess.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		// fmt.Println(v.Name)
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, *guildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	s.AddHandler(shadowban)

	defer s.Close()
	fmt.Println("Bot is online!")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	if *RemoveCommands {
		log.Println("Removing commands...")
		// registeredCommands, err := s.ApplicationCommands(s.State.User.ID, *guildID)
		// if err != nil {
		// 	log.Fatalf("Could not fetch registered commands: %v", err)
		// }

		for _, v := range registeredCommands {
			err := s.ApplicationCommandDelete(s.State.User.ID, *guildID, v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}

	log.Println("Gracefully shutting down.")
}

func shadowban(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if _, ok := shadowbannedUserIDs[m.Author.ID]; ok && m.GuildID == *guildID {
		s.ChannelMessageDelete(m.ChannelID, m.ID)
	}

}
