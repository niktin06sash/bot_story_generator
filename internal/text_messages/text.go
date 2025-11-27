package text_messages

import (
	"bot_story_generator/internal/models"
	"fmt"
	"sort"
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

var TextCommandForAdmin = []textCommandForHelp{
	{
		Command: "/viewSetting",
		Text:    "👁 Просмотр настроек ",
	},
	{
		Command: "/changeSetting",
		Text:    "⚙️ Изменение настроек",
	},
	{
		Command: "/rebootCache",
		Text:    "♻️ Перезагрузка кэша",
	},
	{
		Command: "/addsub",
		Text:    "➕ Добавить подписку: добавляет новую подписку пользователю. Формат: /addsub _=userID type currency price duration_days",
	},
	{
		Command: "/updatesub",
		Text:    "✏️ Обновить подписку: обновляет существующую подписку пользователя. Формат: /updatesub _=userID duration_days",
	},
	{
		Command: "/getsub",
		Text:    "🔍 Посмотреть подписки: возвращает активные подписки пользователя. Формат: /getsub _=userID",
	},
}

// не будем конкретную ошибку писать в тг - просто сообщение-предупреждение
var TextErrorSettings = "❌ Ошибка при взаимодействии с данными настройки. Проверьте логи!"
var TextSuccessSetSetting = "✅ Настройка успешно изменена"
var SuccessActivateSub = "✅ Подписка успешно активирована пользователю: %v"
var SuccessUpdateSub = "✅ Подписка успешно обновлена пользователю: %v"
var SuccessRebootCache = "✅ Кэш успешно перезагружен"

// здесь сплита нет - передается единое сообщение
var TextStartCreateHero = `
🎲 Начинаем новое приключение!  
Сначала создай своего персонажа —  
ведь каждая история начинается с героя.`

var TextErrorCreateTask = `
❗ Судьба героя пока не решена...  
Похоже, магия дала сбой.  
Попробуй выбрать действие ещё раз чуть позже.`
var TextSubPriceError = "Ошибка: сумма подписки не настроена. Пожалуйста, свяжитесь с поддержкой."

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

func TextAdmin() string {
	resp := "👑 Админ-команды:\n"
	for _, cmd := range TextCommandForAdmin {
		resp += cmd.Command + " - " + cmd.Text + "\n"
	}
	return resp
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

func CreateExtensionMessageInDataBase(ext *models.Extension) string {
	var msg string
	msg += ext.Narrative + "\n"
	return msg
}

var TextSupportInfo = `Заглушка support`

var TextTermsOfService = `УСЛОВИЯ ПОЛЬЗОВАНИЯ TELEGRAM-БОТОМ “TaleWeaver AI”
Дата вступления в силу: 27 ноября 2025 года

1. Общие положения
1.1. Настоящие Условия регулируют использование Telegram-бота TaleWeaver AI (далее — «Бот»), разработанного Разработчиком.
1.2. Использование Бота означает полное принятие Условий. При несогласии пользователь обязан прекратить использование.
1.3. Бот функционирует исключительно внутри платформы Telegram и подчиняется правилам Telegram, включая:
• Telegram Terms of Service
• Telegram Bot Terms
• Telegram Stars Terms
1.4. Основные определения:
• «Ход» — единица использования сервиса, расходуемая при определённых действиях.
• «Подписка» — единоразовый доступ к расширенным лимитам ходов, приобретаемый через Telegram Stars.
1.5. Сгенерированный Ботом контент не является объектом интеллектуальной собственности пользователя и используется только в рамках функционала Бота.
1.6. Возрастной рейтинг — 16+. Контент может включать насилие, мрачные темы, грубые выражения или взрослые намёки.
1.7. Генерация осуществляется алгоритмическими моделями; контент может содержать ошибки, предвзятость, неточности или непредсказуемые элементы.

2. Описание услуг
2.1. Бот предоставляет интерактивное создание историй. Пользователь выбирает персонажа, после чего Бот генерирует повествование и варианты продолжения.
2.2. Бесплатный режим: до 10 ходов в сутки.
2.3. Подписка: до 50 ходов в сутки в течение 30 дней с момента покупки.
2.4. Стоимость действий:
• Создание новой истории — расходует 2 хода.
• Выбор действия в истории (одно продолжение) — расходует 1 ход.
2.5. Лимиты ходов и условия использования могут быть изменены в любое время без уведомления.
2.6. Материалы предоставляются «как есть» и могут содержать непредсказуемый или случайный контент.
2.7. Разработчик вправе изменять функциональность без уведомления.

3. Подписка и платежи
3.1. Подписка приобретается только через Telegram Stars.
3.2. Подписка действует 30 дней с момента покупки и не продлевается автоматически.
3.3. Каждая покупка — отдельная транзакция.
3.4. Возврат средств через Разработчика невозможен. Все вопросы по платежам решаются через:
• встроенный механизм споров Telegram,
• поддержку Telegram.

4. Ограничения использования
4.1. Бот доступен пользователям 16+.
4.2. Запрещается:
• обход лимитов ходов,
• использование дополнительных аккаунтов или автоматизации,
• вмешательство в работу Бота,
• попытки эксплуатации или нанесения вреда,
• использование Бота в коммерческих целях.
4.3. При нарушении Условий Разработчик вправе ограничить доступ без компенсаций.
4.4. Пользователь несёт ответственность за свои действия в Telegram и в Боте.

5. Обработка данных
5.1. Сбор минимально необходимых данных:
• Telegram ID,
• история действий и выборов,
• статус подписки.
5.2. Данные используются только для работы Бота и не передаются третьим лицам, кроме Telegram.

6. Ответственность
6.1. Разработчик не отвечает за:
• сбои Telegram,
• технические проблемы связи,
• недоступность сервиса,
• потерю данных,
• ошибки платежей,
• характер и содержание генерируемых историй.
6.2. Пользователь принимает риски, связанные с автоматически генерируемым контентом.
6.3. Максимальная ответственность Разработчика ограничена суммой последней покупки подписки, если Telegram примет решение о возврате.

7. Изменения и прекращение работы
7.1. Разработчик вправе обновлять Условия. Продолжение использования означает согласие.
7.2. Разработчик вправе прекратить работу Бота без обязательств по возвратам.
7.3. Пользователь может прекратить использование в любой момент. Лимиты и Stars не компенсируются.

8. Споры
8.1. Все споры по платежам, Stars и техническим ошибкам рассматриваются исключительно Telegram.
8.2. Разработчик не участвует в финансовых или юридических процессах между Telegram и пользователем.

9. Контакты
Вопросы принимаются через раздел «Поддержка» внутри Бота.
`

// придумать текст
var InvalidPaymentData = "Invalid Payment Data"

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

func FormatSettingsComparison(cacheSettings map[string]string, dbSettings map[string]string) string {
	var resp string

	resp += Divider
	resp += "⚙️ Текущие настройки системы\n"
	resp += Divider

	// Get all unique keys
	keyMap := make(map[string]bool)
	for key := range cacheSettings {
		keyMap[key] = true
	}
	for key := range dbSettings {
		keyMap[key] = true
	}

	// Sort keys for consistent output
	var keys []string
	for key := range keyMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Display each setting
	for _, key := range keys {
		cacheVal, cacheExists := cacheSettings[key]
		dbVal, dbExists := dbSettings[key]

		resp += fmt.Sprintf("🔹 %s\n", key)

		if cacheExists && dbExists {
			if cacheVal == dbVal {
				resp += fmt.Sprintf("   ✅ Значение: %s\n", cacheVal)
			} else {
				resp += fmt.Sprintf("   ⚠️ Кеш: %s\n", cacheVal)
				resp += fmt.Sprintf("   ⚠️ БД: %s\n", dbVal)
				resp += "   🔴 РАССИНХРОНИЗАЦИЯ!\n"
			}
		} else if cacheExists {
			resp += fmt.Sprintf("   ✅ Значение: %s (только в кеше)\n", cacheVal)
		} else if dbExists {
			resp += fmt.Sprintf("   ⚠️ Значение: %s (только в БД, не в кеше!)\n", dbVal)
		} else {
			resp += "   ❌ Нет данных\n"
		}
	}

	resp += Divider

	// Summary
	var cacheOnly, dbOnly, mismatched int
	for key := range keyMap {
		cacheVal, cacheExists := cacheSettings[key]
		dbVal, dbExists := dbSettings[key]

		if cacheExists && !dbExists {
			cacheOnly++
		} else if !cacheExists && dbExists {
			dbOnly++
		} else if cacheExists && dbExists && cacheVal != dbVal {
			mismatched++
		}
	}

	resp += "📊 Статус синхронизации:\n"
	if cacheOnly > 0 {
		resp += fmt.Sprintf("   ⚠️ Только в кеше: %d\n", cacheOnly)
	}
	if dbOnly > 0 {
		resp += fmt.Sprintf("   ⚠️ Только в БД: %d\n", dbOnly)
	}
	if mismatched > 0 {
		resp += fmt.Sprintf("   🔴 Рассинхронизировано: %d\n", mismatched)
	}
	if cacheOnly == 0 && dbOnly == 0 && mismatched == 0 {
		resp += "   ✅ Кеш синхронизирован с БД\n"
	}

	resp += Divider
	return resp
}

func FormatActiveSubscriptionsText(subs []*models.Subscription) string {
	if len(subs) == 0 {
		return "У пользователя нет активных подписок."
	}
	resp := "📦 Активные подписки пользователя:\n"
	for i, sub := range subs {
		resp += fmt.Sprintf("\n№%d:\n", i+1)
		resp += fmt.Sprintf("Тип: %s\n", sub.Type)
		resp += fmt.Sprintf("Старт: %s\n", sub.StartDate.Format("2006-01-02"))
		resp += fmt.Sprintf("Окончание: %s\n", sub.EndDate.Format("2006-01-02"))
		resp += fmt.Sprintf("Цена: %d %s\n", sub.Price, sub.Currency)
		resp += fmt.Sprintf("Статус: %s\n", sub.Status)
	}
	return resp
}

func FormatFinalStory(story string) string {
	resp := Divider
	resp += "🏁 Финал истории\n"
	resp += Divider

	resp += fmt.Sprintf("%s\n", story)

	resp += Divider
	resp += "✨ Чтобы начать новое приключение, используйте команду /newstory"
	resp += Divider

	return resp
}
