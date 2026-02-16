# Bot Story Generator

Telegram bot for generating interactive fantasy stories in the style of Dungeons & Dragons 5E using artificial intelligence.

## 👥 Developers

- https://github.com/niktin06sash
- https://github.com/LeykMihail

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

1. **Story Creation**: User runs `/newstory`
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

1. **Input Layer** (`internal/tg_bot/`)
   - Receive updates from Telegram API and normalize them into internal messages (`readIncomingMessage`)
   - Route commands and payment events into internal queues (`AddComand`, `AddPaymentQuery`)
2. **Routing Layer** (`internal/router/`)
   - Run router workers: `routerWorker` (game flow, admin, settings) and `paymentWorker` (pre-checkout and subscription handling)
   - Maintain per-user processing state and tracing context to avoid concurrent conflicts
3. **Business Logic Layer** (`internal/service/`)
   - `story_service.go` — story creation, continuation and stopping logic
   - `user_service.go` — user registration and limits initialization
   - `subscription_service.go` — subscription lifecycle and payment validation
   - `setting_service.go` — runtime configuration management (limits, prices, etc.)
   - `admin_service.go` — administrative commands and maintenance operations
4. **Infrastructure Layer** 
   - `internal/ai/` — AI client for generating heroes and story segments
   - `internal/database/` — PostgreSQL connection pool and configuration
   - `internal/cache/` — Redis client and connection management
   - `internal/repository/` - Database and Cache logic
5. **Output Layer** (`internal/tg_bot/`)
   - Sends outbound messages, edits, deletions, invoices and payment responses back to Telegram
   - Maps internal models (`OutboundMessage`, `EditMessage`, `DeleteMessage`, `InvoiceMessage`, `PaymentData`) to Telegram API calls
   - Runs dedicated workers for outbound, edit, delete, payment and invoice channels
   
---
![Architecture Diagram](./images/architecture.png)
---

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

## 🚀 Deployment & Local Setup

### Production Deployment

The project uses **Railway** for deployment:
- **PostgreSQL** — database deployed on Railway
- **Redis** — cache deployed on Railway

### Local Setup

To run the project locally using Docker and the provided `Makefile`, follow these steps:

#### Prerequisites

- Docker and Docker CLI
- `make`
- `migrate` CLI (`migrate` command) for applying database migrations
- PostgreSQL (local instance or external, e.g. Railway)
- Redis (local instance or external, e.g. Railway)
- (Optional) Go 1.21 or higher — only if you want to run tests locally without Docker

#### 1. Clone the repository

```bash
git clone https://github.com/niktin06sash/bot_story_generator
cd bot_story_generator
```

#### 2. Configure application environment (`cfg.env`)

Create a `cfg.env` file in the project root based on the example below (without secrets):

```cfg.env
# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
TELEGRAM_BOT_DEBUG=True
TELEGRAM_BOT_OFFSET=0
TELEGRAM_BOT_TIMEOUT=60

# AI Configuration
AI_API_KEY=your_openrouter_api_key_here
AI_MODEL=deepseek/deepseek-chat-v3-0324
AI_CONNECT_TIMEOUT=180s
AI_COMPLETION_TIMEOUT=180s
AI_PATH_PROMT_MAIN_GAME_RULES=promts/main_game_rules.txt
AI_PATH_PROMT_CREATE_HERO=promts/create_hero.txt
AI_SCHEMAPARAMS_NAME_HEROES=fantasy_characters
AI_SCHEMAPARAMS_DESCRIPTION_HEROES=Array of fantasy characters
AI_SCHEMAPARAMS_NAME_STORYSEGMENT=story_segment
AI_SCHEMAPARAMS_DESCRIPTION_STORYSEGMENT=Narrative + answer options

# Database Configuration (PostgreSQL)
DATABASE_CONNECT_TIMEOUT=60s
DATABASE_CONNECT_URL=postgresql://username:password@localhost:5432/dbname

# Cache Configuration (Redis)
CACHE_URL=redis://localhost:6379
CACHE_USER_CREATED_KEY=user_created:%d
CACHE_EXCEEDED_LIMIT_KEY=limit_exceeded:%d
CACHE_CONNECT_TIMEOUT=30s
CACHE_SETTINGS_KEY=settings:%s

# Application Settings
TOKEN_DAY_LIMIT=100
PREMIUM_TOKEN_DAY_LIMIT=10000
PRICE_BASIC_SUBSCRIPTION=1
NUM_WORKERS=16

# Logger Configuration
LOGGER_INFO_FILE_PATH=internal/logger/logs/info.log
LOGGER_WARN_FILE_PATH=internal/logger/logs/warn.log
LOGGER_ERROR_FILE_PATH=internal/logger/logs/error.log
LOGGER_DEBUG_FILE_PATH=internal/logger/logs/debug.log

# Admin Configuration
ADMIN_IDS=your_telegram_user_id_here
```

#### 3. Configure `.env` for migrations and Makefile

The `Makefile` includes and exports variables from `.env` for running migrations.  
Create a `.env` file in the project root, for example:

```env
DATABASE_CONNECT_URL=postgresql://username:password@localhost:5432/dbname
```

#### 4. Apply database migrations

Migrations are located in `internal/schema` and are applied via `make`:

```bash
# Apply all pending migrations
make migrate-up

# Roll back the last migration (optional)
make migrate-down
```

#### 5. Build and run the application with Docker

Use the `Makefile` targets to build the Docker image and run the container:

```bash
# Build Docker image
make build

# Start the container
make start
```

Or run the full pipeline (build → tests → migrations → start) with a single command:

```bash
make run
```

Additional useful commands:

```bash
# View container logs
make logs

# Check container status
make status

# Stop the container
make stop
```

#### 6. Verify the setup

Open Telegram and find your bot. Send the `/start` command to verify it is working.


**Enjoy the game! 🎲✨**
