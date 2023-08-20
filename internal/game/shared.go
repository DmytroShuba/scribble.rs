package game

import (
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
)

//
// This file contains all structs and constants that are shared with clients.
// These files require special treatment, since we use easyjson for marshalling
// and unmarshalling. If this file changes, run:
//
//  easyjson -all ./internal/game/shared.go
//

// Eevnts that are just incomming from the client.
const (
	EventTypeStart          = "start"
	EventTypeRequestDrawing = "request-drawing"
	EventTypeChooseWord     = "choose-word"
	EventTypeUndo           = "undo"
)

// Events that are outgoing only.
const (
	EventTypeUpdatePlayers            = "update-players"
	EventTypeUpdateWordHint           = "update-wordhint"
	EventTypeCorrectGuess             = "correct-guess"
	EventTypeCloseGuess               = "close-guess"
	EventTypeSystemMessage            = "system-message"
	EventTypeNonGuessingPlayerMessage = "non-guessing-player-message"
	EventTypeReady                    = "ready"
	EventTypeGameOver                 = "game-over"
	EventTypeYourTurn                 = "your-turn"
	EventTypeNextTurn                 = "next-turn"
	EventTypeDrawing                  = "drawing"
	EventTypeDrawerKicked             = "drawer-kicked"
	EventTypeOwnerChange              = "owner-change"
	EventTypeLobbySettingsChanged     = "lobby-settings-changed"
	EventTypeShutdown                 = "shutdown"
)

// Events that are bidirectional.
var (
	EventTypeKickVote          = "kick-vote"
	EventTypeNameChange        = "name-change"
	EventTypeMessage           = "message"
	EventTypeLine              = "line"
	EventTypeFill              = "fill"
	EventTypeClearDrawingBoard = "clear-drawing-board"
)

type State string

const (
	// Unstarted means the lobby has been opened but never started.
	Unstarted State = "unstarted"
	// Ongoing means the lobby has already been started.
	Ongoing State = "ongoing"
	// GameOver means that the lobby had been start, but the max round limit
	// has already been reached.
	GameOver State = "gameOver"
)

// Event contains an eventtype and optionally any data.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type StringDataEvent struct {
	Data string `json:"data"`
}

type EventTypeOnly struct {
	Type string `json:"type"`
}

type IntDataEvent struct {
	Data int `json:"data"`
}

// WordHint describes a character of the word that is to be guessed, whether
// the character should be shown and whether it should be underlined on the
// UI.
type WordHint struct {
	Character rune `json:"character"`
	Underline bool `json:"underline"`
}

// RGBColor represents a 24-bit color consisting of red, green and blue.
type RGBColor struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
}

type LineEvent struct {
	Data struct {
		FromX     float32  `json:"fromX"`
		FromY     float32  `json:"fromY"`
		ToX       float32  `json:"toX"`
		ToY       float32  `json:"toY"`
		Color     RGBColor `json:"color"`
		LineWidth float32  `json:"lineWidth"`
	} `json:"data"`
	Type string `json:"type"`
}

type FillEvent struct {
	Data *struct {
		X     float32  `json:"x"`
		Y     float32  `json:"y"`
		Color RGBColor `json:"color"`
	} `json:"data"`
	Type string `json:"type"`
}

// KickVote represents a players vote to kick another players. If the VoteCount
// is as great or greater than the RequiredVoteCount, the event indicates a
// successful kick vote. The voting is anonymous, meaning the voting player
// won't be exposed.
type KickVote struct {
	PlayerID          uuid.UUID `json:"playerId"`
	PlayerName        string    `json:"playerName"`
	VoteCount         int       `json:"voteCount"`
	RequiredVoteCount int       `json:"requiredVoteCount"`
}

type OwnerChangeEvent struct {
	PlayerID   uuid.UUID `json:"playerId"`
	PlayerName string    `json:"playerName"`
}

type NameChangeEvent struct {
	PlayerID   uuid.UUID `json:"playerId"`
	PlayerName string    `json:"playerName"`
}

// GameOverEvent is basically the ready event, but contains the last word.
// This is required in order to show the last player the word, in case they
// didn't manage to guess it in time. This is necessary since the last word
// is usually part of the "next-turn" event, which we don't send, since the
// game is over already.
type GameOverEvent struct {
	*Ready
	PreviousWord string `json:"previousWord"`
}

// NextTurn represents the data necessary for displaying the lobby state right
// after a new turn started. Meaning that no word has been chosen yet and
// therefore there are no wordhints and no current drawing instructions.
type NextTurn struct {
	// PreviousWord signals the last chosen word. If empty, no word has been
	// chosen. The client can now themselves whether there has been a previous
	// turn, by looking at the current gamestate.
	PreviousWord string    `json:"previousWord"`
	Players      []*Player `json:"players"`
	Round        int       `json:"round"`
	RoundEndTime int       `json:"roundEndTime"`
}

// OutgoingMessage represents a message in the chatroom.
type OutgoingMessage struct {
	// Author is the player / thing that wrote the message
	Author string `json:"author"`
	// AuthorID is the unique identifier of the authors player object.
	AuthorID uuid.UUID `json:"authorId"`
	// Content is the actual message text.
	Content string `json:"content"`
}

// Ready represents the initial state that a user needs upon connection.
// This includes all the necessary things for properly running a client
// without receiving any more data.
type Ready struct {
	PlayerID     uuid.UUID `json:"playerId"`
	PlayerName   string    `json:"playerName"`
	AllowDrawing bool      `json:"allowDrawing"`

	VotekickEnabled    bool        `json:"votekickEnabled"`
	GameState          State       `json:"gameState"`
	OwnerID            uuid.UUID   `json:"ownerId"`
	Round              int         `json:"round"`
	Rounds             int         `json:"rounds"`
	RoundEndTime       int         `json:"roundEndTime"`
	DrawingTimeSetting int         `json:"drawingTimeSetting"`
	WordHints          []*WordHint `json:"wordHints"`
	Players            []*Player   `json:"players"`
	CurrentDrawing     []any       `json:"currentDrawing"`
}

// Player represents a participant in a Lobby.
type Player struct {
	// userSession uniquely identifies the player.
	userSession      uuid.UUID
	ws               *websocket.Conn
	socketMutex      *sync.Mutex
	lastKnownAddress string
	// disconnectTime is used to kick a player in case the lobby doesn't have
	// space for new players. The player with the oldest disconnect.Time will
	// get kicked.
	disconnectTime *time.Time
	// desiredState is used for state changes between spectator and player.
	// We want to prevent people from switching in and out of the play state.
	// While this will allow people to skip being the drawer, it will also
	// cause them to lose points for that round.
	desiredState PlayerState

	votedForKick map[uuid.UUID]bool

	// ID uniquely identified the Player.
	ID uuid.UUID `json:"id"`
	// Name is the players displayed name
	Name string `json:"name"`
	// Score is the points that the player got in the current Lobby.
	Score int `json:"score"`
	// Connected defines whether the players websocket connection is currently
	// established. This has previously been in state but has been moved out
	// in order to avoid losing the state on refreshing the page.
	// While checking the websocket against nil would be enough, we still need
	// this field for sending it via the APIs.
	Connected bool `json:"connected"`
	// Rank is the current ranking of the player in his Lobby
	LastScore int         `json:"lastScore"`
	Rank      int         `json:"rank"`
	State     PlayerState `json:"state"`
}

// EditableLobbySettings represents all lobby settings that are editable by
// the lobby owner after the lobby has already been opened.
type EditableLobbySettings struct {
	// MaxPlayers defines the maximum amount of players in a single lobby.
	MaxPlayers int `json:"maxPlayers"`
	// CustomWords are additional words that will be used in addition to the
	// predefined words.
	// Public defines whether the lobby is being broadcast to clients asking
	// for available lobbies.
	Public bool `json:"public"`
	// EnableVotekick decides whether players are allowed to kick eachother
	// by casting majority votes.
	EnableVotekick bool `json:"enableVotekick"`
	// CustomWordsChance determines the chance of each word being a custom
	// word on the next word prompt. This needs to be an integer between
	// 0 and 100. The value represents a percentage.
	CustomWordsChance int `json:"customWordsChance"`
	// ClientsPerIPLimit helps preventing griefing by reducing each player
	// to one tab per IP address.
	ClientsPerIPLimit int `json:"clientsPerIpLimit"`
	// DrawingTime is the amount of seconds that each player has available to
	// finish their drawing.
	DrawingTime int `json:"drawingTime"`
	// Rounds defines how many iterations a lobby does before the game ends.
	// One iteration means every participant does one drawing.
	Rounds int `json:"rounds"`
}
