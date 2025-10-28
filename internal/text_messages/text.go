package text_messages

import (
	"bot_story_generator/internal/models"
	"fmt"
	"strings"
)

const Divider = "━━━━━━━━━━━━━━━━━━━━\n"

var TextGreeting = `Привет! 👋
Я — твой проводник в мире интерактивных историй.
Я рассказываю истории, а ты решаешь, как поступит герой.

⚔️ Выбирай путь, спасай друзей, обманывай врагов или ищи любовь — всё зависит только от тебя.

Хочешь начать новое приключение?
👉 Напиши /newstory
📜 Или /help, если хочешь узнать, как всё устроено.`

type textCommandForHelp struct {
	Command string
	Text    string
}

var TextCommandForHelp = []textCommandForHelp{
	{
		Command: "/start",
		Text:    "Вызвать бота",
	},
	{
		Command: "/newstory",
		Text:    "Начать новое приключение",
	},
	{
		Command: "/stopstory",
		Text:    "Завершить текущую историю",
	},
	{
		Command: "/help",
		Text:    "Посмотреть список команд",
	},
}

// здесь сплита нет - передается единое сообщение
var TextStartCreateHero = `
🎲 Начинаем новое приключение!  
Сначала создай своего персонажа —  
ведь каждая история начинается с героя.`

var TextErrorCreateTask = `
❗ Судьба героя пока не решена...  
Похоже, магия дала сбой.  
Попробуй выбрать действие ещё раз чуть позже.`

// надо будет поменять ответ
var TextErrorUserActiveStory = `Уже есть активная история. Сначала завершите текущую!`

// надо будет поменять ответ
var TextStopActiveStory = `Вы действительно хотите завершить текущую историю?`

// надо будет поменять ответ
var TextNoActiveStory = `У вас нет активной истории. Воспользуйтесь командой /newstory`

// надо будет поменять ответ
var TextSuccessStopStory = `Вы успешно завершили историю. Для создания новой воспользуйтесь кнопкой /newstory`

// надо будет поменять ответ
var TextErrorUserDailyLimit = `Превышен лимит дневных ходов. Возвращайтесь завтра!`

func TextHelp() string {
	text := "Вот список команд:\n"
	for _, command := range TextCommandForHelp {
		text += command.Command + " - " + command.Text + "\n"
	}
	return text
}

// --- символ для сплита при анимации
var WaitingTextHeroes = `🪶 Придумываю твою историю...---
⚙️ Создаю героев...---
📜 Переплетаю сюжетные линии...---
🌌 Добавляю немного магии...`

var WaitingTextNarrative = `⚔️ Герои собираются с духом перед новым испытанием...---
	🛡 Судьба взвешивает твой следующий шаг...---
	🌪 Ветры перемен кружат над полем сражения...---
	🔥 Клинки звенят, готовясь к решающему удару...---
	🌄 Рассвет уже близко — история ждёт продолжения...---
`

func TextNarrativeWithChoices(narrative string, choices []string) string {
	resp := ""
	resp += Divider
	if narrative != "" {
		resp += fmt.Sprintf("🧭 Развитие событий:\n%s\n\n", narrative)
	}
	resp += Divider
	if len(choices) > 0 {
		resp += "⚡ Выбор действий:\n"
		for i, choice := range choices {
			resp += fmt.Sprintf("%d️⃣ %s\n", i+1, choice)
		}
	}
	resp += Divider
	return resp
}

func FormatHeroDescription(h models.Hero) string {
	var resp string
	resp += Divider
	resp += "🧝‍♂️Твой герой создан!\n"
	resp += Divider

	resp += fmt.Sprintf("🪪 Имя: %s\n", h.Name)
	resp += fmt.Sprintf("🌍 Раса: %s\n", h.Race)
	resp += fmt.Sprintf("⚔️ Класс: %s\n", h.Class)

	resp += "\n👁️ Внешность:\n"
	resp += h.Appearance + "\n"

	if len(h.Traits) > 0 {
		resp += "\n💠 Черты характера:\n"
		for _, t := range h.Traits {
			resp += "• " + t + "\n"
		}
	}

	resp += "\n✨ Особенность:\n"
	resp += h.Feature + "\n"

	resp += "\n📜 Биография:\n"
	resp += h.Biography + "\n"

	resp += Divider

	return resp
}

func NewChouseHero(heroes *models.FantasyCharacters) []string {
	resp := make([]string, len(heroes.Characters))
	for idx, hero := range heroes.Characters {
		str := Divider
		str += fmt.Sprintf("🧙‍♂️ Персонаж #%d\n", idx+1)
		str += Divider
		if hero.Name != "" {
			str += fmt.Sprintf("🏷️ Имя: %s\n", hero.Name)
		}
		if hero.Race != "" {
			str += fmt.Sprintf("🧬 Раса: %s\n", hero.Race)
		}
		if hero.Class != "" {
			str += fmt.Sprintf("⚔️ Класс: %s\n", hero.Class)
		}
		if hero.Appearance != "" {
			str += fmt.Sprintf("🪞 Внешность: %s\n", hero.Appearance)
		}
		if len(hero.Traits) > 0 {
			str += "💭 Черты характера: "
			for i, trait := range hero.Traits {
				if i > 0 {
					str += ", "
				}
				str += trait
			}
			str += "\n"
		}
		if hero.Feature != "" {
			str += fmt.Sprintf("✨ Особенность: %s\n", hero.Feature)
		}
		if hero.Biography != "" {
			str += fmt.Sprintf("📜 Биография: %s\n", hero.Biography)
		}
		str += Divider
		resp[idx] = str
	}
	resp[len(resp)-1] = "🌟 Выберите своего героя из представленных вариантов\n"
	return resp
}

func CreateHeroMessage(hero *models.Hero) string {
	var msg string
	msg += Divider
	msg += "🏰 Характеристика персонажа\n"
	msg += Divider

	if hero.Name != "" {
		msg += fmt.Sprintf("🏷️ Имя: %s\n", hero.Name)
	}
	if hero.Race != "" {
		msg += fmt.Sprintf("🧬 Раса: %s\n", hero.Race)
	}
	if hero.Class != "" {
		msg += fmt.Sprintf("⚔️ Класс: %s\n", hero.Class)
	}
	if hero.Appearance != "" {
		msg += fmt.Sprintf("🪞 Внешность: %s\n", hero.Appearance)
	}
	if len(hero.Traits) > 0 {
		msg += "💭 Черты характера: "
		msg += strings.Join(hero.Traits, ", ")
		msg += "\n"
	}
	if hero.Feature != "" {
		msg += fmt.Sprintf("✨ Особенность: %s\n", hero.Feature)
	}
	if hero.Biography != "" {
		msg += fmt.Sprintf("📜 Биография:\n%s\n", hero.Biography)
	}

	msg += Divider
	return msg
}

func CreateExtensionMessage(ext *models.Extension) string {
	var msg string
	msg += Divider
	msg += "📖 Продолжение истории\n"
	msg += Divider
	msg += ext.Narrative
	msg += Divider
	return msg
}