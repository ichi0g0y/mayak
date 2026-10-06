package model

import "time"

type LogEntry struct {
	ID        uint64    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Category  string    `json:"category"`
	Message   string    `json:"message"`
}

// UpdateStatus is the state of the check for a newer MAYAK on GitHub
// Releases (internal/app/app_update.go). State is idle, checking, current,
// available, downloading (Progress in percent), ready (downloaded, installed
// on quit or restart), unsupported (no build for Platform) or error.
type UpdateStatus struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	State       string `json:"state"`
	Progress    int    `json:"progress"`
	Platform    string `json:"platform"`
	ReleaseURL  string `json:"releaseUrl"`
	ReleaseName string `json:"releaseName"`
	Notes       string `json:"notes"`
	PublishedAt string `json:"publishedAt"`
	CheckedAt   string `json:"checkedAt"`
	LastError   string `json:"lastError"`
}

type Position struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Z          float64 `json:"z"`
	Rotation   float64 `json:"rotation"`
	DetectedAt string  `json:"detectedAt"`
}

// PlayerProfile is what the character's Overall screen showed when last
// read (internal/profiledetect): the nickname, the level, the experience,
// and the stats beside the level. Picture: a picture of the character's
// panel was kept with it (profile.jpg).
type PlayerProfile struct {
	Name         string  `json:"name,omitempty"`
	Level        int     `json:"level,omitempty"`
	Exp          int     `json:"exp,omitempty"`
	Raids        int     `json:"raids,omitempty"`
	Kills        int     `json:"kills,omitempty"`
	SurvivalRate float64 `json:"survivalRate,omitempty"`
	KD           float64 `json:"kd,omitempty"`
	Hours        float64 `json:"hours,omitempty"`
	At           string  `json:"at"`
	Picture      bool    `json:"picture,omitempty"`
}

// CompletableTask is a task a Tasks screenshot showed completed in the
// game that TarkovTracker does not have completed.
type CompletableTask struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Trader string `json:"trader"`
}

type Status struct {
	// CompletableTasks are the tasks done in the game, as Tasks screenshots
	// showed, not completed on TarkovTracker: for the player to apply.
	CompletableTasks []CompletableTask `json:"completableTasks,omitempty"`
	// Profile is the character's Overall screen as last read.
	Profile           *PlayerProfile   `json:"profile,omitempty"`
	Hideout           HideoutStatus    `json:"hideout"`
	Catalog           CatalogStatus    `json:"catalog"`
	Connection        string           `json:"connection"`
	Monitoring        bool             `json:"monitoring"`
	CurrentMap        string           `json:"currentMap"`
	RaidActive        bool             `json:"raidActive"`
	RaidStartedAt     string           `json:"raidStartedAt"`
	RunThroughAt      string           `json:"runThroughAt"`
	LastQueueSeconds  float64          `json:"lastQueueSeconds"`
	LastScreenshot    string           `json:"lastScreenshot"`
	ScreenshotType    string           `json:"screenshotType"`
	Position          *Position        `json:"position,omitempty"`
	LastError         string           `json:"lastError"`
	DetectionScore    float64          `json:"detectionScore"`
	DetectionLayout   string           `json:"detectionLayout"`
	AnalysisStage     string           `json:"analysisStage"`
	OCRRaw            string           `json:"ocrRaw"`
	LastQuest         string           `json:"lastQuest"`
	QuestID           string           `json:"questId"`
	QuestTrader       string           `json:"questTrader"`
	QuestMap          string           `json:"questMap"`
	MatchConfidence   float64          `json:"matchConfidence"`
	QuestCandidates   []QuestCandidate `json:"questCandidates"`
	CropPreview       string           `json:"cropPreview"`
	QuestURL          string           `json:"questUrl"`
	QuestWikiURL      string           `json:"questWikiUrl"`
	QuestObjectives   []QuestObjective `json:"questObjectives"`
	LastItem          string           `json:"lastItem"`
	ItemID            string           `json:"itemId"`
	ItemShortName     string           `json:"itemShortName"`
	ItemURL           string           `json:"itemUrl"`
	ItemIconURL       string           `json:"itemIconUrl"`
	ItemConfidence    float64          `json:"itemConfidence"`
	ItemCandidates    []ItemCandidate  `json:"itemCandidates"`
	LastRemoteCommand string           `json:"lastRemoteCommand"`
	Tracker           TrackerStatus    `json:"tracker"`
}

type CatalogStatus struct {
	State               string `json:"state"`
	Mode                string `json:"mode"`
	UpdatedAt           string `json:"updatedAt"`
	Items               int    `json:"items"`
	Maps                int    `json:"maps"`
	Traders             int    `json:"traders"`
	Tasks               int    `json:"tasks"`
	HideoutStations     int    `json:"hideoutStations"`
	ScavCooldownSeconds int    `json:"scavCooldownSeconds"`
	PlayerLevels        int    `json:"playerLevels"`
	LastError           string `json:"lastError"`
}

type TrackerStatus struct {
	Connection  string `json:"connection"`
	Mode        string `json:"mode"`
	ProfileID   string `json:"profileId"`
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	// UserID is the TarkovTracker user the token belongs to (its /token
	// owner): their shared profile is tarkovtracker.org/profile/<id>/<mode>.
	UserID             string `json:"userId"`
	PlayerLevel        int    `json:"playerLevel"`
	CompletedTasks     int    `json:"completedTasks"`
	FailedTasks        int    `json:"failedTasks"`
	PVPConfigured      bool   `json:"pvpConfigured"`
	PVEConfigured      bool   `json:"pveConfigured"`
	SeasonalConfigured bool   `json:"seasonalConfigured"`
	// GameRunning: Escape from Tarkov is running (past logs are not rechecked then).
	GameRunning bool                    `json:"gameRunning"`
	LastSync    string                  `json:"lastSync"`
	LastEvent   string                  `json:"lastEvent"`
	LastError   string                  `json:"lastError"`
	Keys        []TrackerKeySummary     `json:"keys"`
	Profiles    []TrackerProfileSummary `json:"profiles"`
}

type TrackerKeySummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Mode        string `json:"mode"`
	MaskedToken string `json:"maskedToken"`
	AccountID   string `json:"accountId"`
	ProfileID   string `json:"profileId"`
	Bound       bool   `json:"bound"`
}

type TrackerProfileSummary struct {
	AccountID  string `json:"accountId"`
	ProfileID  string `json:"profileId"`
	Mode       string `json:"mode"`
	FirstSeen  string `json:"firstSeen"`
	LastSeen   string `json:"lastSeen"`
	BoundKeyID string `json:"boundKeyId"`
	Current    bool   `json:"current"`
	// HistorySyncedAt is when its past logs were last synced, empty when never.
	HistorySyncedAt string `json:"historySyncedAt"`
	// HistoryFrom is the day its past logs are read from, empty for all.
	HistoryFrom string `json:"historyFrom"`
}

type ItemCandidate struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ShortName  string  `json:"shortName"`
	URL        string  `json:"url"`
	IconURL    string  `json:"iconUrl"`
	Confidence float64 `json:"confidence"`
}

type QuestObjective struct {
	Description string   `json:"description"`
	Maps        []string `json:"maps"`
}

type QuestCandidate struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Trader     string  `json:"trader"`
	Map        string  `json:"map"`
	Confidence float64 `json:"confidence"`
}
