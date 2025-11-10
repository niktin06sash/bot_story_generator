package text_messages

import (
	"bot_story_generator/internal/models"
	"fmt"
	"time"
	// "strings"
)

const Divider = "━━━━━━━━━━━━━━━━━━━━\n"

var TextGreeting = `Привет! 👋
Я — твой проводник в мире интерактивных историй.
Я рассказываю истории, а ты решаешь, как поступит герой.

⚔️ Выбирай путь, спасай друзей, обманывай врагов или ищи любовь — всё зависит только от тебя.

Хочешь начать новое приключение?
👉 Напиши /newstory
📜 Или /help, если хочешь узнать, как всё устроено.`

var TextUnknownCommand = `Такой команды нет. Воспользуйся /help, если хочешь узнать, как всё устроено.`

type textCommandForHelp struct {
	Command string
	Text    string
}

var TextCommandForHelp = []textCommandForHelp{
	{
		Command: "/start",
		Text:    "📜 Пробудить хроники — призови бота и начни свой путь.",
	},
	{
		Command: "/newstory",
		Text:    "🗺️ Начать новое приключение — создай историю, полную магии и выборов.",
	},
	{
		Command: "/stopstory",
		Text:    "⛔ Завершить текущую историю — оборви нить судьбы и начни заново.",
	},
	{
		Command: "/help",
		Text:    "💬 Список команд — напоминание для странника, ищущего путь.",
	},
	{
		Command: "/subscription",
		Text:    "💎 Статус подписки — узнай о своих возможностях и преимуществах.",
	},
	{
		Command: "/buySubscription",
		Text:    "⭐ Получить подписку — открой доступ к премиум-возможностям для ещё большего погружения.",
	},
	{
		Command: "/terms",
		Text:    "📄 Пользовательское соглашение — подробнее о правилах сервиса.",
	},
	{
		Command: "/support",
		Text:    "🆘 Поддержка — связь с магистрами, которые готовы помочь.",
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

var TextErrorUserActiveStory = `Вы уже вплетены в нить текущего приключения. Если хотите оборвать её, используйте команду /stopstory.`

var TextStopActiveStory = `Желаете прервать своё текущее путешествие? Всё, что пережито, останется в памяти хроник.`

var TextNoActiveStory = `Пока ваша книга приключений пуста. Воспользуйтесь командой /newstory, чтобы начать новый путь.`

var TextSuccessStopStory = `История завершена. Перо судьбы готово писать новую главу — воспользуйтесь /newstory.`

// Подбадривание к покупке подписки — коротко и с призывом к действию
var TextErrorUserDailyLimit = `Ваши силы на сегодня иссякли. Отдохните и возвращайтесь завтра, чтобы продолжить свой путь.
Хотите продолжить прямо сейчас? Оформите подписку — получите дополнительные ходы, эксклюзивный контент и приоритет.
👉 Оформить подписку: /buySubscription`

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

func NewChouseHero(heroes *models.FantasyCharacters) []string {
	// Создаем массив на 1 элемент больше для текста выбора
	resp := make([]string, len(heroes.Characters)+1)
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
	// Текст выбора в последнем элементе, не перезаписывая персонажей
	resp[len(heroes.Characters)] = "🌟 Выберите своего героя из представленных вариантов\n"
	return resp
}

func CreateHeroMessage(h *models.Hero) string {
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

func CreateExtensionMessage(ext *models.Extension) string {
	var msg string
	msg += Divider
	msg += "📖 Вы выбрали:\n"
	msg += Divider
	msg += ext.Narrative + "\n"
	msg += Divider
	return msg
}

var TextSupportInfo = `Заглушка support`

var TextTermsOfService = `Заглушка terms`

// придумать текст
var InvalidPaymentData = "Invalid Payment"
var TextSendInvoiceSubscription = `💫 Счёт на оплату отправлен!
Следуйте инструкциям Telegram, чтобы завершить покупку подписки.`

var TextSubscriptionActivated = `🌟 Подписка активирована!
Ваши возможности выросли — теперь вы можете создавать гораздо больше историй!`

var TextErrorProcessPayment = `🚫 Произошла ошибка при инициализации платежа.
Свяжитесь с поддержкой через /support, чтобы решить проблему.`

var TextErrorActivateSubscription = `⚠️ Не удалось активировать подписку.
Пожалуйста, обратитесь в поддержку через /support.`

var TextAlreadyActiveSubscription = `💫 Подписка уже активна. Повторная покупка недоступна.`

func CreateSubscriptionStatusMessage(typeSub string, startDate, endDate time.Time) string {
	var resp string

	resp += Divider
	resp += "💎 Статус подписки\n"
	resp += Divider

	if typeSub == "" {
		resp += "🚫 Подписка не активна.\n"
		resp += "Вы можете оформить её командой /buySubscription.\n"
		resp += Divider
		return resp
	}

	resp += fmt.Sprintf("📦 Тип подписки: %s\n", typeSub)
	resp += fmt.Sprintf("📅 Дата активации: %s\n", startDate.Format("02.01.2006"))
	resp += fmt.Sprintf("⏳ Действует до: %s\n", endDate.Format("02.01.2006"))

	daysLeft := int(time.Until(endDate).Hours() / 24)
	if daysLeft > 0 {
		resp += fmt.Sprintf("🕒 Осталось дней: %d\n", daysLeft)
	} else {
		resp += "⚠️ Срок действия подписки истёк.\n"
	}

	resp += Divider
	return resp
}

func CreateNoSubscriptionMessage() string {
	var resp string

	resp += Divider
	resp += "💎 Статус подписки\n"
	resp += Divider

	resp += "🚫 У вас нет активной подписки.\n"
	resp += "Чтобы открыть доступ к созданию историй без ограничений — оформите подписку.\n\n"
	resp += "💠 Доступные команды:\n"
	resp += "• /buySubscription — оформить подписку\n"
	resp += "• /help — список доступных команд\n"

	resp += Divider
	return resp
}

var TextErrorGetSubscriptionStatus = `⚠️ Не удалось получить статус подписки.`
