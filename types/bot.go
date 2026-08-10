package types

type Bot struct {
	Config *BotConfig
	User   *UserConfig
}

func (c *Bot) DoChat(ask string) *Message {
	return &Message{
		Reply:  "",
		Status: 200,
	}
}

func (c *Bot) DoQuery(md string) *Message {
	return &Message{
		Reply:  "",
		Status: 200,
	}
}
