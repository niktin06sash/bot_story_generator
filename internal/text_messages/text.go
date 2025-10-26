package text_messages

import (
	"bot_story_generator/internal/models"
	"fmt"
)

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
	resp += "━━━━━━━━━━━━━━━━━━━━\n"
	if narrative != "" {
		resp += fmt.Sprintf("🧭 Развитие событий:\n%s\n\n", narrative)
	}
	resp += "━━━━━━━━━━━━━━━━━━━━\n"
	if len(choices) > 0 {
		resp += "⚡ Выбор действий:\n"
		for i, choice := range choices {
			resp += fmt.Sprintf("%d️⃣ %s\n", i+1, choice)
		}
	}
	resp += "━━━━━━━━━━━━━━━━━━━━\n"
	return resp
}

func FormatHeroDescription(h models.Hero) string {
	var resp string
	resp += "━━━━━━━━━━━━━━━━━━━━\n"
	resp += "🧝‍♂️Твой герой создан!\n"
	resp += "━━━━━━━━━━━━━━━━━━━━\n"

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

	resp += "━━━━━━━━━━━━━━━━━━━━"

	return resp
}

func NewChouseHero(heroes *models.FantasyCharacters) []string {
	resp := make([]string, len(heroes.Characters))
	// resp := "🌟 Выберите своего героя из представленных вариантов:\n"
	for idx, hero := range heroes.Characters {
		str := "───────────────────────\n"
		str += fmt.Sprintf("🧙‍♂️ Персонаж #%d\n", idx+1)
		str += "───────────────────────\n"
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
		str += "───────────────────────\n"
		resp[idx] = str
	}
	resp[len(resp)-1] = "🌟 Выберите своего героя из представленных вариантов\n"
	return resp
}
