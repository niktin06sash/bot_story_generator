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

var TextStartCreateHero = `
🎲 Начинаем новое приключение!  
Сначала создай своего персонажа —  
ведь каждая история начинается с героя.`

var TextErrorCreateHero = `
❗ Судьба героя пока не решена...  
Похоже, магия дала сбой.  
Попробуй создать персонажа ещё раз чуть позже.`

func TextChooseHero(heroes *models.FantasyCharacters) string {
	resp := "🌟 *Выберите своего героя из представленных вариантов:*\n\n"
	for idx, hero := range heroes.Characters {
		resp += fmt.Sprintf("🧙‍♂️ *Персонаж #%d*\n", idx+1)
		resp += "───────────────────────\n"
		if hero.Name != "" {
			resp += fmt.Sprintf("🏷️ *Имя:* %s\n", hero.Name)
		}
		if hero.Race != "" {
			resp += fmt.Sprintf("🧬 *Раса:* %s\n", hero.Race)
		}
		if hero.Class != "" {
			resp += fmt.Sprintf("⚔️ *Класс:* %s\n", hero.Class)
		}
		if hero.Appearance != "" {
			resp += fmt.Sprintf("🪞 *Внешность:* %s\n", hero.Appearance)
		}
		if len(hero.Traits) > 0 {
			resp += "💭 *Черты характера:* "
			for i, trait := range hero.Traits {
				if i > 0 {
					resp += ", "
				}
				resp += trait
			}
			resp += "\n"
		}
		if hero.Feature != "" {
			resp += fmt.Sprintf("✨ *Особенность:* %s\n", hero.Feature)
		}
		if hero.Biography != "" {
			resp += fmt.Sprintf("📜 *Биография:* %s\n", hero.Biography)
		}
		if hero.Tone != "" {
			resp += fmt.Sprintf("🎭 *Тон:* %s\n", hero.Tone)
		}
		resp += "\n───────────────────────\n\n"
	}
	return resp
}
func TextHelp() string {
	text := "Вот список команд:\n"
	for _, command := range TextCommandForHelp {
		text += command.Command + " - " + command.Text + "\n"
	}
	return text
}
