package handlers

import (
	"fmt"
	"net/url"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/webauthn"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/models"
	"github.com/authelia/authelia/v4/internal/session"
)

func getWebAuthnUser(ctx *middlewares.AutheliaCtx, userSession session.UserSession) (user *models.WebauthnUser, err error) {
	user = &models.WebauthnUser{
		Username:    userSession.Username,
		DisplayName: userSession.DisplayName,
	}

	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}

	if user.Devices, err = ctx.Providers.StorageProvider.LoadWebauthnDevicesByUsername(ctx, userSession.Username); err != nil {
		return nil, err
	}

	return user, nil
}

func newWebauthn(ctx *middlewares.AutheliaCtx) (w *webauthn.WebAuthn, err error) {
	var (
		u *url.URL
	)

	if u, err = ctx.GetOriginalURL(); err != nil {
		return nil, err
	}

	rpID := u.Hostname()
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	config := &webauthn.Config{
		RPDisplayName: ctx.Configuration.Webauthn.DisplayName,
		RPID:          rpID,
		RPOrigin:      origin,
		RPIcon:        "",

		AttestationPreference: ctx.Configuration.Webauthn.ConveyancePreference,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.CrossPlatform,
			UserVerification:        ctx.Configuration.Webauthn.UserVerification,
			RequireResidentKey:      protocol.ResidentKeyUnrequired(),
		},

		Timeout: int(ctx.Configuration.Webauthn.Timeout.Milliseconds()),
	}

	ctx.Logger.Tracef("Creating new Webauthn RP instance with ID %s and Origin %s", config.RPID, config.RPOrigin)

	return webauthn.New(config)
}
