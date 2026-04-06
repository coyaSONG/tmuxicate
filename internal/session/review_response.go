package session

import (
	"fmt"

	"github.com/coyaSONG/tmuxicate/internal/mailbox"
	"github.com/coyaSONG/tmuxicate/internal/protocol"
)

func ReviewRespond(stateDir string, store *mailbox.Store, agent string, reviewMessageID protocol.MessageID, outcome protocol.ReviewOutcome, body []byte) (protocol.MessageID, error) {
	return "", fmt.Errorf("review respond not implemented yet")
}
