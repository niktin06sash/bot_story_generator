# Bot Story Generator

Telegram bot for generating interactive fantasy stories in the style of Dungeons & Dragons 5E using artificial intelligence.

## 📋 Description

**Bot Story Generator** is a Telegram bot that allows users to create and play interactive text adventures in the D&D 5E style. The bot uses AI to generate unique characters, create dynamic storylines, and manage gameplay in real-time.

### Key Features

- 🎭 **Character Creation**: Generation of unique fantasy characters using AI (race, class, biography, characteristics)
- 📖 **Interactive Stories**: Dynamic story creation with action choices (5 options per step)
- 🎲 **D&D 5E Mechanics**: Full support for D&D 5E rules, including dice rolls, skills, saving throws
- 💎 **Subscription System**: Monetization through Telegram Stars (XTR) with various plans
- ⚡ **Caching**: Performance optimization through Redis
- 📊 **Admin Panel**: Settings management and system monitoring
- 🔒 **Limits**: Daily token limits system for free and premium users

## 🏗️ Architecture

The project is built on clean architecture with layer separation:

```
bot_story_generator/
├── cmd/app/              # Application entry point
├── internal/
│   ├── ai/              # AI integration (OpenRouter/OpenAI)
│   ├── cache/           # Redis caching
│   ├── config/          # Application configuration
│   ├── database/        # PostgreSQL connection
│   ├── logger/          # Structured logging (zap)
│   ├── models/          # Data models and JSON schemas
│   ├── repository/      # Data access layer
│   ├── router/          # Command routing
│   ├── schema/         # Database migrations
│   ├── service/         # Business logic
│   ├── text_messages/   # Bot text messages
│   ├── tg_bot/          # Telegram Bot API integration
│   └── tracing/         # Request tracing
├── promts/              # AI prompts
└── Dockerfile           # Docker image
```

## 📚 Usage

### Gameplay

1. **Story Creation**: User runs `/create_story`
2. **Character Selection**: Bot generates 5 unique characters using AI
3. **Adventure Begins**: After selecting a character, the story begins
4. **Interactivity**: At each step, the user chooses one of 5 actions
5. **Continuation**: The story develops dynamically based on user choices

### Limits System

- **Free users**: 100 tokens per day
- **Premium users**: 10,000 tokens per day
- Limits reset every 24 hours

## 🛠️ Development

### Architecture Layers

1. **Presentation Layer** (`internal/tg_bot/`, `internal/router/`)
   - Telegram message handling
   - Command routing
   - Response sending

2. **Business Logic Layer** (`internal/service/`)
   - `story_service.go` — story creation and management logic
   - `user_service.go` — user management
   - `subscription_service.go` — subscription and payment processing
   - `setting_service.go` — settings management
   - `admin_service.go` — administrative commands

3. **Data Access Layer** (`internal/repository/`)
   - PostgreSQL operations
   - Redis caching
   - Transactions

4. **Infrastructure Layer**
   - `internal/ai/` — AI API integration
   - `internal/database/` — database connection
   - `internal/cache/` — Redis connection
   - `internal/logger/` — logging

## 🔧 Configuration

### Environment Variables

#### Telegram
- `TELEGRAM_BOT_TOKEN` — bot token (required)
- `TELEGRAM_BOT_DEBUG` — debug mode (True/False)
- `TELEGRAM_BOT_OFFSET` — offset for receiving updates
- `TELEGRAM_BOT_TIMEOUT` — timeout for receiving updates (seconds)

#### AI
- `AI_API_KEY` — OpenRouter/OpenAI API key
- `AI_MODEL` — AI model (e.g., `deepseek/deepseek-chat-v3-0324`)
- `AI_CONNECT_TIMEOUT` — AI connection timeout
- `AI_COMPLETION_TIMEOUT` — request completion timeout

#### Database
- `DATABASE_CONNECT_URL` — PostgreSQL connection string
- `DATABASE_CONNECT_TIMEOUT` — connection timeout

#### Cache
- `CACHE_URL` — Redis connection string
- `CACHE_CONNECT_TIMEOUT` — connection timeout

#### Application
- `TOKEN_DAY_LIMIT` — daily token limit for free users
- `PREMIUM_TOKEN_DAY_LIMIT` — daily limit for premium users
- `PRICE_BASIC_SUBSCRIPTION` — basic subscription price (in Stars/XTR)
- `NUM_WORKERS` — number of workers for message processing
- `ADMIN_IDS` — administrator IDs (comma-separated)

## 📊 Database

### Database Schema

The project uses migrations for database schema management:

- `001_users_table` — users table
- `002_stories_table` — stories table
- `003_storiesmessages_table` — story messages table
- `004_storiesvariants_table` — action variants table
- `005_dailyLimits_table` — daily limits table
- `006_subscriptions_table` — subscriptions table
- `007_settings_table` — settings table

## 🔐 Security

- All sensitive data is stored in environment variables
- API keys are not committed to the repository
- Structured logging without secret leakage
- Validation of all incoming data

## 📝 Logging

The project uses structured logging via `zap`:

- **Info** — informational messages
- **Debug** — debug information
- **Warn** — warnings
- **Error** — errors

Logs are written to files in the `internal/logger/logs/` directory.

Each message includes:
- `traceID` — unique request identifier
- `userID` — user ID
- `executionTime` — operation execution time

## 🧪 Testing

Uses `testify` library for assertions and `go.uber.org/mock` for mocks.

---

**Enjoy the game! 🎲✨**
