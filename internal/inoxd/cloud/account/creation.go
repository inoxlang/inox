package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/core"
)

const (
	CHALLENGE_ATTEMPT_TIMEOUT    = time.Minute
	HOSTER_API_REQUEST_TIMEOUT   = 5 * time.Second
	MAX_HOSTER_API_RESPONSE_SIZE = 20_000
	TOKEN_ACK_WAIT_TIMEOUT       = 2 * time.Second
)

type Connection struct {
	PrintFn  func(text string)
	ReadChan chan string
}

var (
	ErrAccountCreationFailed                       = errors.New("account creation failed")
	ErrAccountCreationFailedInternalError          = fmt.Errorf("%w: internal error", ErrAccountCreationFailed)
	ErrAccountCreationFailedUsernameDoesNotMatch   = fmt.Errorf("%w: username does not match", ErrAccountCreationFailed)
	ErrAccountCreationFailedChallValueDoesNotMatch = fmt.Errorf("%w: challenge value does not match", ErrAccountCreationFailed)
)

// CreateDisposableAccountInteractively communicates with conn to create a disposable account interactively.
func CreateDisposableAccountInteractively(ctx *core.Context, hosterName string, conn *Connection, db *DisposableAccountDatabase) error {
	hoster, err := getProofHosterByName(hosterName)
	if err != nil {
		return err
	}

	//tell the user what is the challenge.
	challValue, err := randomChallengeValue()
	if err != nil {
		return err
	}

	explanationTemplate := PROOF_HOSTER_CHALLENGE_EXPLANATION_TEMPLATES[hoster]
	buf := bytes.NewBuffer(nil)
	if err := explanationTemplate.Execute(buf, challengeTemplateContext{ChallValue: challValue}); err != nil {
		return fmt.Errorf("%w: %w", ErrAccountCreationFailed, err)
	}

	explanation := buf.String()
	conn.PrintFn("explanation:" + explanation)

	var username string

	//wait for the user to attempt the challenge.
	select {
	case <-time.After(CHALLENGE_ATTEMPT_TIMEOUT):
		return fmt.Errorf("%w: time out", ErrAccountCreationFailed)
	case <-ctx.Done():
		return ctx.Err()
	case username = <-conn.ReadChan:
	}

	//compute the proof's location
	buf.Reset()
	err = PROOF_LOCATION_TEMPLATES[hoster].Execute(buf, proofLocationTemplateContext{
		Username: username,
	})
	if err != nil {
		return ErrAccountCreationFailedInternalError
	}

	proofLocation := buf.String()

	//make a request to the hoster.
	req, err := http.NewRequest("GET", proofLocation, nil)
	if err != nil {
		return ErrAccountCreationFailedInternalError
	}
	reqCtx, cancel := context.WithTimeout(ctx, HOSTER_API_REQUEST_TIMEOUT)
	defer cancel()
	req = req.WithContext(reqCtx)

	resp, err := http.DefaultClient.Do(req)
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("%w: error while getting the proof from %s: %w", ErrAccountCreationFailed, hosterName, err)
	}

	body := io.LimitReader(resp.Body, MAX_HOSTER_API_RESPONSE_SIZE) //prevent large reads.
	content, err := io.ReadAll(body)
	if err != nil {
		return ErrAccountCreationFailedInternalError
	}

	var userIdOnHoster string

	//check the proof
	switch hoster {
	default:
		return ErrUnsupportedProofHoster
	case Github:
		var apiResponse struct {
			Owner struct {
				Id    int    `json:"id"`
				Type  string `json:"type"`
				Login string `json:"login"`
			} `json:"owner"`
			Description string    `json:"description"`
			UpdatedAt   time.Time `json:"updated_at"`
		}

		err = json.Unmarshal(content, &apiResponse)
		if err != nil {
			return fmt.Errorf("%w: failed to parse API response from hoster: %w", ErrAccountCreationFailed, err)
		}

		if apiResponse.Owner.Type != "User" {
			return fmt.Errorf("%w: repository should be owned by a user", ErrAccountCreationFailed)
		}

		if apiResponse.Owner.Login != username {
			return ErrAccountCreationFailedUsernameDoesNotMatch
		}

		description := strings.TrimSpace(apiResponse.Description)
		if description != challValue {
			return ErrAccountCreationFailedChallValueDoesNotMatch
		}

		userIdOnHoster = strconv.Itoa(apiResponse.Owner.Id)
	}

	account, hexEncodedToken, err := NewDisposableAccount(DisposableAccountCreation{
		Hoster:           hoster,
		UserIdOnHoster:   userIdOnHoster,
		UsernameOnHoster: username,
	})

	if err != nil {
		return fmt.Errorf("%w: %w", ErrAccountCreationFailed, err)
	}

	conn.PrintFn("token:" + hexEncodedToken)

	var ack string
	select {
	case ack = <-conn.ReadChan:
		if ack != "ack:token" {
			return fmt.Errorf("%w: token reception was not acknowledged", ErrAccountCreationFailed)
		}
	case <-time.After(TOKEN_ACK_WAIT_TIMEOUT):
		return fmt.Errorf("%w: time out", ErrAccountCreationFailed)
	}

	err = db.Persist(ctx, account)
	if err != nil {
		return fmt.Errorf("%w: failed to persist new account", ErrAccountCreationFailed)
	}
	return nil
}
