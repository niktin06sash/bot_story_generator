package text_messages

var TextGreeting = `Привет! 👋
Я — твой проводник в мире интерактивных историй.
Я рассказываю истории, а ты решаешь, как поступит герой.

⚔️ Выбирай путь, спасай друзей, обманывай врагов или ищи любовь — всё зависит только от тебя.

Хочешь начать новое приключение?
👉 Напиши /newstory
📜 Или /help, если хочешь узнать, как всё устроено.`

type textCommandForHelp struct {
	Command string
	Text string
}

var TextCommandForHelp = []textCommandForHelp{
	{
		Command: "/start",
		Text: "Вызвать бота",
	},
	{
		Command: "/newstory",
		Text: "Начать новое приключение",
	},
	{
		Command: "/help",
		Text: "Посмотреть список команд",
	},
}