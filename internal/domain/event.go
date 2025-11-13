package domain

import (
	"time"

	"github.com/google/uuid"
)

// EventLogRecord represents a single record from 1C Event Log (Журнал регистрации)
// Fields match the structure shown in 1C Configurator UI
type EventLogRecord struct {
	// Основные поля (Primary View)
	EventTime time.Time // Дата, время
	EventDate time.Time // Дата (без времени)

	// Идентификация базы/кластера
	ClusterGUID  string // GUID кластера
	ClusterName  string // Имя кластера (из cluster_map.yaml)
	InfobaseGUID string // GUID информационной базы
	InfobaseName string // Имя информационной базы (из cluster_map.yaml)

	// Основная информация о событии
	Level              string // Уровень (Information, Warning, Error, Note)
	Event              string // Событие (внутренний код, например _$InfoBase$_.Update)
	EventPresentation  string // Событие (представление, например "Данные. Изменение")

	// Пользователь и компьютер
	UserName string    // Пользователь (имя)
	UserID   uuid.UUID // UUID пользователя
	Computer string    // Компьютер

	// Приложение
	Application             string // Приложение (код: ThinClient, ThickClient, WebClient)
	ApplicationPresentation string // Приложение (представление: "Тонкий клиент")

	// Сеанс и соединение
	SessionID    uint64 // Сеанс (номер)
	ConnectionID uint64 // Соединение

	// Транзакция
	TransactionStatus string // Статус транзакции (Зафиксирована, Отменена, Не завершена)
	TransactionID     string // Идентификатор транзакции

	// Разделение данных сеанса
	DataSeparation string // Разделение данных сеанса (0, 1, ...)

	// Метаданные
	MetadataName         string // Метаданные (полное имя, например Document.CommercialOffer)
	MetadataPresentation string // Метаданные (представление, например "Документ. Коммерческое предложение")

	// Детальная информация
	Comment          string // Комментарий
	Data             string // Данные (технические)
	DataPresentation string // Представление данных (ссылка на объект)

	// Сервер (для клиент-серверного варианта)
	Server        string // Рабочий сервер
	PrimaryPort   uint16 // Основной IP порт
	SecondaryPort uint16 // Вспомогательный IP порт

	// Дополнительные свойства
	Properties map[string]string // Расширяемые свойства
}

