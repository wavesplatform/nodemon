package clients

import (
	"context"
	stderrs "errors"

	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/userpass"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	vaultTokenTTLIncrement = 3600 // in seconds
)

func NewVaultSimpleClient(ctx context.Context, logger *zap.Logger, addr, user, pass string) (*vault.Client, error) {
	config := vault.DefaultConfig()
	if _, err := config.ParseAddress(addr); err != nil { // change and check the vault address
		return nil, errors.Wrap(err, "failed to parse vault address")
	}
	if err := config.Error; err != nil {
		return nil, errors.Wrap(err, "failed to create vault config")
	}

	client, clErr := vault.NewClient(config)
	if clErr != nil {
		return nil, errors.Wrap(clErr, "failed to create vault client from config")
	}

	if _, loginErr := vaultLogin(ctx, client, user, pass); loginErr != nil { // check for creds and other stuff
		return nil, errors.Wrap(loginErr, "failed to initially login to vault")
	}
	go renewToken(ctx, logger, client, user, pass) // run renewal goroutine
	return client, nil
}

var errWatcherRenewFailed = errors.New("token renewal failed")

// Once you've set the token for your Vault client, you will need to
// periodically renew its lease.
//
// A function like this should be run as a goroutine to avoid blocking.
//
// Production applications may also wish to be more tolerant of failures and
// retry rather than exiting.
//
// Additionally, enterprise Vault users should be aware that due to eventual
// consistency, the API may return unexpected errors when running Vault with
// performance standbys or performance replication, despite the client having
// a freshly renewed token. See https://www.vaultproject.io/docs/enterprise/consistency#vault-1-7-mitigations
// for several ways to mitigate this which are outside the scope of this code sample.
func renewToken(ctx context.Context, logger *zap.Logger, client *vault.Client, user, pass string) {
	for {
		if ctx.Err() != nil { // context done
			return
		}
		vaultLoginResp, loginErr := vaultLogin(ctx, client, user, pass)
		if loginErr != nil && !errors.Is(loginErr, context.Canceled) {
			logger.Fatal("unable to authenticate to Vault", zap.Error(loginErr))
		}
		logger.Info("Successfully authenticated to Vault", zap.String("request_id", vaultLoginResp.RequestID))
		tokenErr := manageTokenLifecycle(ctx, logger, client, vaultLoginResp)
		if tokenErr != nil && !errors.Is(tokenErr, context.Canceled) && !errors.Is(tokenErr, errWatcherRenewFailed) {
			logger.Fatal("unable to start managing token lifecycle", zap.Error(tokenErr))
		}
	}
}

// Starts token lifecycle management. Returns only fatal errors as errors,
// otherwise returns nil, so we can attempt login again.
func manageTokenLifecycle(ctx context.Context, logger *zap.Logger, client *vault.Client, token *vault.Secret) error {
	// You may notice a different top-level field called Renewable.
	// That one is used for dynamic secrets renewal, not token renewal.
	if renew := token.Auth.Renewable; !renew {
		logger.Warn("Token is not configured to be renewable. Re-attempting login.")
		return nil
	}

	watcher, err := client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret: token,
		// Learn more about this optional value
		// in https://www.vaultproject.io/docs/concepts/lease#lease-durations-and-renewal
		Increment: vaultTokenTTLIncrement,
	})
	if err != nil {
		return errors.Wrap(err, "unable to initialize new lifetime watcher for renewing auth token")
	}

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		// `DoneCh` will return if renewal fails, or if the remaining lease
		// duration is under a built-in threshold and either renewing is not
		// extending it or renewing is disabled. In any case, the caller
		// needs to attempt to log in again.
		case renewErr := <-watcher.DoneCh():
			if renewErr != nil {
				logger.Error("Failed to renew vault token. Re-attempting login.", zap.Error(renewErr))
				return stderrs.Join(errWatcherRenewFailed, renewErr)
			}
			// This occurs once the token has reached max TTL.
			logger.Info("Vault token can no longer be renewed. Re-attempting login.")
			return nil

		// Successfully completed renewal
		case renewal := <-watcher.RenewCh():
			logger.Info("Vault token successfully renewed", zap.String("request_id", renewal.Secret.RequestID))
		}
	}
}

func vaultLogin(ctx context.Context, client *vault.Client, user, pass string) (*vault.Secret, error) {
	// WARNING: A plaintext password like this is obviously insecure.
	// See the files in the auth-methods directory for full examples of how to securely
	// log in to Vault using various auth methods. This function is just
	// demonstrating the basic idea that a *vault.Secret is returned by
	// the login call.
	userpassAuth, err := auth.NewUserpassAuth(user, &auth.Password{FromString: pass})
	if err != nil {
		return nil, errors.Wrap(err, "unable to initialize userpass auth method")
	}

	authInfo, err := client.Auth().Login(ctx, userpassAuth)
	if err != nil {
		return nil, errors.Wrap(err, "unable to login to userpass auth method")
	}
	if authInfo == nil {
		return nil, errors.New("no auth info was returned after login")
	}

	return authInfo, nil
}
