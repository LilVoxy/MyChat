package load

import (
	"database/sql"

	"github.com/LilVoxy/coursework_chat/ETL/models"
	"github.com/LilVoxy/coursework_chat/ETL/utils"
)

// Loader интерфейс для загрузки данных в OLAP
type Loader interface {
	// LoadUserDimension загружает данные в измерение пользователей
	LoadUserDimension(users []models.UserDimension) error

	// LoadMessageFacts загружает данные в факты сообщений
	LoadMessageFacts(messages []models.MessageFact) error

	// LoadChatFacts загружает данные в факты чатов
	LoadChatFacts(chats []models.ChatFact) error

	// LoadDailyActivityFacts загружает данные в факты ежедневной активности
	LoadDailyActivityFacts(facts []models.DailyActivityFact) error

	// LoadHourlyActivityFacts загружает данные в факты почасовой активности
	LoadHourlyActivityFacts(facts []models.HourlyActivityFact) error
}

// OLAPLoader реализация Loader для OLAP базы данных
type OLAPLoader struct {
	db     *sql.DB
	logger *utils.ETLLogger

	// Загрузчики для отдельных типов данных
	userLoader     *UserLoader
	messageLoader  *MessageLoader
	chatLoader     *ChatLoader
	activityLoader *ActivityLoader
}

// NewOLAPLoader создает новый экземпляр OLAPLoader
func NewOLAPLoader(db *sql.DB, logger *utils.ETLLogger) *OLAPLoader {
	loader := &OLAPLoader{
		db:     db,
		logger: logger,
	}

	// Инициализация загрузчиков для отдельных типов данных
	loader.userLoader = NewUserLoader(db, logger)
	loader.messageLoader = NewMessageLoader(db, logger)
	loader.chatLoader = NewChatLoader(db, logger)
	loader.activityLoader = NewActivityLoader(db, logger)

	return loader
}

// LoadUserDimension загружает данные в измерение пользователей
func (l *OLAPLoader) LoadUserDimension(users []models.UserDimension) error {
	return l.userLoader.Load(users)
}

// LoadMessageFacts загружает данные в факты сообщений
func (l *OLAPLoader) LoadMessageFacts(messages []models.MessageFact) error {
	return l.messageLoader.Load(messages)
}

// LoadChatFacts загружает данные в факты чатов
func (l *OLAPLoader) LoadChatFacts(chats []models.ChatFact) error {
	return l.chatLoader.Load(chats)
}

// LoadDailyActivityFacts загружает данные в факты ежедневной активности
func (l *OLAPLoader) LoadDailyActivityFacts(facts []models.DailyActivityFact) error {
	return l.activityLoader.LoadDailyFacts(facts)
}

// LoadHourlyActivityFacts загружает данные в факты почасовой активности
func (l *OLAPLoader) LoadHourlyActivityFacts(facts []models.HourlyActivityFact) error {
	return l.activityLoader.LoadHourlyFacts(facts)
}
