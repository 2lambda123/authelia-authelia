package handlers

import (
	"bytes"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/webauthn"

	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/models"
	"github.com/authelia/authelia/v4/internal/regulation"
)

// SecondFactorWebauthnAssertionGET handler starts the assertion ceremony.
func SecondFactorWebauthnAssertionGET(ctx *middlewares.AutheliaCtx) {
	var (
		w     *webauthn.WebAuthn
		user  *models.WebauthnUser
		appid string
		err   error
	)

	userSession := ctx.GetSession()

	if w, appid, err = getWebauthn(ctx); err != nil {
		ctx.Logger.Errorf("Unable to configire %s during assertion challenge for user '%s': %+v", regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if user, err = getWebAuthnUser(ctx, userSession); err != nil {
		ctx.Logger.Errorf("Unable to create %s assertion challenge for user '%s': %+v", regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	var opts []webauthn.LoginOption

	extensions := make(map[string]interface{})

	if user.HasFIDOU2F() {
		extensions["appid"] = appid
	}

	if len(extensions) != 0 {
		opts = append(opts, webauthn.WithAssertionExtensions(extensions))
	}

	var assertion *protocol.CredentialAssertion

	if assertion, userSession.Webauthn, err = w.BeginLogin(user, opts...); err != nil {
		ctx.Logger.Errorf("Unable to create %s assertion challenge for user '%s': %+v", regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if err = ctx.SaveSession(userSession); err != nil {
		ctx.Logger.Errorf(logFmtErrSessionSave, "assertion challenge", regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if err = ctx.SetJSONBody(assertion); err != nil {
		ctx.Logger.Errorf(logFmtErrWriteResponseBody, regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}
}

// SecondFactorWebauthnAssertionPOST handler completes the assertion ceremony after verifying the challenge.
func SecondFactorWebauthnAssertionPOST(ctx *middlewares.AutheliaCtx) {
	var (
		err error
		w   *webauthn.WebAuthn

		requestBody signWebauthnRequestBody
	)

	if err = ctx.ParseBody(&requestBody); err != nil {
		ctx.Logger.Errorf(logFmtErrParseRequestBody, regulation.AuthTypeWebauthn, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	userSession := ctx.GetSession()

	if w, _, err = getWebauthn(ctx); err != nil {
		ctx.Logger.Errorf("Unable to configire %s during assertion challenge for user '%s': %+v", regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	var (
		assertionResponse *protocol.ParsedCredentialAssertionData
		credential        *webauthn.Credential
		user              *models.WebauthnUser
		saved             bool
	)

	if assertionResponse, err = protocol.ParseCredentialRequestResponseBody(bytes.NewReader(ctx.PostBody())); err != nil {
		ctx.Logger.Errorf("Unable to parse %s assertionfor user '%s': %+v", regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if user, err = getWebAuthnUser(ctx, userSession); err != nil {
		ctx.Logger.Errorf("Unable to load %s devices for assertion challenge for user '%s': %+v", regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if credential, err = w.ValidateLogin(user, *userSession.Webauthn, assertionResponse); err != nil {
		_ = markAuthenticationAttempt(ctx, false, nil, userSession.Username, regulation.AuthTypeWebauthn, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	for _, device := range user.Devices {
		if bytes.Equal(device.KID, credential.ID) {
			device.SignCount = credential.Authenticator.SignCount

			if err = ctx.Providers.StorageProvider.UpdateWebauthnDeviceSignCount(ctx, device); err != nil {
				ctx.Logger.Errorf("Unable to save %s device signin count for assertion challenge for user '%s' device '%x' count '%d': %+v", regulation.AuthTypeWebauthn, userSession.Username, credential.ID, credential.Authenticator.SignCount, err)
			}

			saved = true

			break
		}
	}

	if !saved {
		ctx.Logger.Errorf("Unable to save %s device signin count for assertion challenge for user '%s' device '%x' count '%d': unable to find device", regulation.AuthTypeWebauthn, userSession.Username, credential.ID, credential.Authenticator.SignCount)
	}

	if err = ctx.Providers.SessionProvider.RegenerateSession(ctx.RequestCtx); err != nil {
		ctx.Logger.Errorf(logFmtErrSessionRegenerate, regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if err = markAuthenticationAttempt(ctx, true, nil, userSession.Username, regulation.AuthTypeWebauthn, nil); err != nil {
		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	userSession.SetTwoFactor(ctx.Clock.Now())
	userSession.Webauthn = nil

	if err = ctx.SaveSession(userSession); err != nil {
		ctx.Logger.Errorf(logFmtErrSessionSave, "removal of the assertion challenge and authentication time", regulation.AuthTypeWebauthn, userSession.Username, err)

		respondUnauthorized(ctx, messageMFAValidationFailed)

		return
	}

	if userSession.OIDCWorkflowSession != nil {
		handleOIDCWorkflowResponse(ctx)
	} else {
		Handle2FAResponse(ctx, requestBody.TargetURL)
	}
}
