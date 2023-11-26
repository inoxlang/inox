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

// CreateAnonymousAccountInteractively communicates with conn to create a disposable account interactively.
func CreateAnonymousAccountInteractively(ctx *core.Context, hosterName string, conn *Connection, db *AnonymousAccountDatabase) error {
	hoster, err := getProofHosterByName(hosterName)
	if err != nil {
		return err
	}

	//tell the user what is the challenge.
	challValue, err := randomChallengeValue()
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(nil)
	var repoName string
	{
		repoNameTemplate := PROOF_REPOSITORY_NAME_TEMPLATES[hoster]

		buf.Reset()
		if err := repoNameTemplate.Execute(buf, ProofRepoNameTemplateContext{ChallValue: challValue}); err != nil {
			return fmt.Errorf("%w: %w", ErrAccountCreationFailed, err)
		}
		repoName = buf.String()
	}

	var explanation string
	{
		explanationTemplate := PROOF_HOSTER_CHALLENGE_EXPLANATION_TEMPLATES[hoster]

		buf.Reset()
		if err := explanationTemplate.Execute(buf, challengeTemplateContext{RepoName: repoName, ChallValue: challValue}); err != nil {
			return fmt.Errorf("%w: %w", ErrAccountCreationFailed, err)
		}

		explanation = buf.String()
		conn.PrintFn("explanation:" + explanation)
	}

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
		Username: username, //we assume case differences in some letters do not matter.
		RepoName: repoName,
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
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: error while getting the proof from %s: HTTP status %d", ErrAccountCreationFailed, hosterName, resp.StatusCode)
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
			return fmt.Errorf("%w: repository should be owned by a user, not a(n) %q", ErrAccountCreationFailed, apiResponse.Owner.Type)
		}

		//check the usernames are the same (case insensitive).
		if !strings.EqualFold(apiResponse.Owner.Login, strings.ToLower(username)) {
			return ErrAccountCreationFailedUsernameDoesNotMatch
		}

		userIdOnHoster = strconv.Itoa(apiResponse.Owner.Id)
	case Gitlab:
		//note: the challenge value is already in the name of the repository.

		var apiResponse struct {
			Namespace struct {
				Id   int    `json:"id"`
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"namespace"`
		}

		err = json.Unmarshal(content, &apiResponse)
		if err != nil {
			return fmt.Errorf("%w: failed to parse API response from hoster: %w", ErrAccountCreationFailed, err)
		}

		if apiResponse.Namespace.Kind != "user" {
			return fmt.Errorf("%w: repository should be owned by a user, not a(n) %q", ErrAccountCreationFailed, apiResponse.Namespace.Kind)
		}

		//check the usernames are the same (case insensitive).
		if !strings.EqualFold(apiResponse.Namespace.Name, strings.ToLower(username)) {
			return ErrAccountCreationFailedUsernameDoesNotMatch
		}

		userIdOnHoster = strconv.Itoa(apiResponse.Namespace.Id)
	}

	account, hexEncodedToken, err := NewDisposableAccount(DisposableAccountCreation{
		Hoster:         hoster,
		UserIdOnHoster: userIdOnHoster,
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
		return fmt.Errorf("%w: failed to persist new account: %w", ErrAccountCreationFailed, err)
	}
	return nil
}
