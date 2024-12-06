package messaging

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	PubSubTopic      = "alerts"
	BotRequestsTopic = "bot_requests"

	numberOfStringsAfterSlash = 2
)

func ParseHostAndPortFromURL(natsPubSubURL string) (string, int, error) {
	// Find the position of "://" and trim everything before it
	withoutProtocol := strings.SplitN(natsPubSubURL, "://", numberOfStringsAfterSlash)
	if len(withoutProtocol) != numberOfStringsAfterSlash {
		return "", 0, fmt.Errorf("failed to split the URL into pieces, URL: %s", natsPubSubURL)
	}

	// Split the remaining part into host and port
	hostPort := strings.Split(withoutProtocol[1], ":")
	if len(hostPort) != numberOfStringsAfterSlash {
		return "", 0, fmt.Errorf("failed to split host port string into host and port, %s", hostPort)
	}

	host := hostPort[0]
	// Convert the port to an integer
	port, err := strconv.Atoi(hostPort[1])
	if err != nil {
		return "", 0, errors.Errorf("failed to parse port, %v", err)
	}
	return host, port, nil
}
