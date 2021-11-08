package handlers

import (
	"fmt"
	"net/url"

	"github.com/authelia/authelia/v4/internal/duo"
	"github.com/authelia/authelia/v4/internal/middlewares"
	"github.com/authelia/authelia/v4/internal/session"
	"github.com/authelia/authelia/v4/internal/utils"
)

// SecondFactorDuoPost handler for sending a push notification via duo api.
func SecondFactorDuoPost(duoAPI duo.API) middlewares.RequestHandler {
	return func(ctx *middlewares.AutheliaCtx) {
		var requestBody signDuoRequestBody

		var device, method string

		if err := ctx.ParseBody(&requestBody); err != nil {
			handleAuthenticationUnauthorized(ctx, err, messageMFAValidationFailed)
			return
		}

		userSession := ctx.GetSession()
		remoteIP := ctx.RemoteIP().String()

		preferredDevice, preferredMethod, err := ctx.Providers.StorageProvider.LoadPreferredDuoDevice(userSession.Username)
		if err != nil {
			ctx.Logger.Debugf("Error identifying preferred device for user %s: %s", userSession.Username, err)
			ctx.Logger.Debugf("Starting Duo PreAuth for initial device selection of user: %s", userSession.Username)
			device, method, err = HandleInitialDeviceSelection(duoAPI, ctx, requestBody.TargetURL)
		} else {
			ctx.Logger.Debugf("Starting Duo PreAuth to check preferred device of user: %s", userSession.Username)
			device, method, err = HandlePreferredDeviceCheck(duoAPI, ctx, preferredDevice, preferredMethod, requestBody.TargetURL)
		}

		if err != nil {
			ctx.Error(err, messageMFAValidationFailed)
			return
		}

		if device == "" || method == "" {
			return
		}

		ctx.Logger.Debugf("Starting Duo Auth attempt for %s with device %s and method %s from IP %s", userSession.Username, device, method, remoteIP)

		values, err := SetValues(userSession, device, method, remoteIP, requestBody.TargetURL, requestBody.Passcode)
		if err != nil {
			handleAuthenticationUnauthorized(ctx, err, messageMFAValidationFailed)
			return
		}

		authResponse, err := duoAPI.AuthCall(values, ctx)
		if err != nil {
			handleAuthenticationUnauthorized(ctx, fmt.Errorf("duo API errored: %s", err), messageMFAValidationFailed)
			return
		}

		if authResponse.Result != allow {
			ctx.ReplyUnauthorized()
			return
		}

		HandleAllow(ctx, requestBody.TargetURL)
	}
}

// HandleInitialDeviceSelection handler for retrieving all available devices.
func HandleInitialDeviceSelection(duoAPI duo.API, ctx *middlewares.AutheliaCtx, targetURL string) (string, string, error) {
	result, message, devices, enrollURL, err := DuoPreAuth(duoAPI, ctx)

	if err != nil {
		handleAuthenticationUnauthorized(ctx, fmt.Errorf("Duo PreAuth API errored: %s", err), messageMFAValidationFailed)
		return "", "", nil
	}

	userSession := ctx.GetSession()

	switch result {
	case enroll:
		ctx.Logger.Debugf("Duo user not enrolled: %s", userSession.Username)

		if err := ctx.SetJSONBody(DuoSignResponse{Result: enroll, EnrollURL: enrollURL}); err != nil {
			return "", "", fmt.Errorf("unable to set JSON body in response")
		}

		return "", "", nil
	case deny:
		ctx.Logger.Infof("Duo user %s not allowed to authenticate: %s", userSession.Username, message)

		if err := ctx.SetJSONBody(DuoSignResponse{Result: deny}); err != nil {
			return "", "", fmt.Errorf("unable to set JSON body in response")
		}

		return "", "", nil
	case allow:
		ctx.Logger.Debugf("Duo authentication was bypassed for user: %s", userSession.Username)
		HandleAllow(ctx, targetURL)

		return "", "", nil
	case auth:
		device, method, err := HandleAutoSelection(ctx, devices, userSession.Username)
		if err != nil {
			return "", "", err
		}

		return device, method, nil
	}

	return "", "", fmt.Errorf("unknown result: %s", result)
}

// HandlePreferredDeviceCheck handler to check if the saved device and method is still valid.
func HandlePreferredDeviceCheck(duoAPI duo.API, ctx *middlewares.AutheliaCtx, device string, method string, targetURL string) (string, string, error) {
	result, message, devices, enrollURL, err := DuoPreAuth(duoAPI, ctx)
	if err != nil {
		handleAuthenticationUnauthorized(ctx, fmt.Errorf("duo PreAuth API errored: %s", err), messageMFAValidationFailed)
		return "", "", nil
	}

	userSession := ctx.GetSession()

	switch result {
	case enroll:
		ctx.Logger.Debugf("Duo user not enrolled: %s", userSession.Username)

		if err := ctx.Providers.StorageProvider.DeletePreferredDuoDevice(userSession.Username); err != nil {
			return "", "", fmt.Errorf("unable to delete preferred Duo device and method for user %s: %s", userSession.Username, err)
		}

		if err := ctx.SetJSONBody(DuoSignResponse{Result: enroll, EnrollURL: enrollURL}); err != nil {
			return "", "", fmt.Errorf("unable to set JSON body in response")
		}

		return "", "", nil
	case deny:
		ctx.Logger.Infof("Duo user %s not allowed to authenticate: %s", userSession.Username, message)
		ctx.ReplyUnauthorized()

		return "", "", nil
	case allow:
		ctx.Logger.Debugf("Duo authentication was bypassed for user: %s", userSession.Username)
		HandleAllow(ctx, targetURL)

		return "", "", nil
	case auth:
		if devices == nil {
			ctx.Logger.Debugf("No compatible device/method available for Duo user: %s", userSession.Username)

			if err := ctx.Providers.StorageProvider.DeletePreferredDuoDevice(userSession.Username); err != nil {
				return "", "", fmt.Errorf("unable to delete preferred Duo device and method for user %s: %s", userSession.Username, err)
			}

			if err := ctx.SetJSONBody(DuoSignResponse{Result: enroll}); err != nil {
				return "", "", fmt.Errorf("unable to set JSON body in response")
			}

			return "", "", nil
		}

		if len(devices) > 0 {
			for i := range devices {
				if devices[i].Device == device {
					if utils.IsStringInSlice(method, devices[i].Capabilities) {
						return device, method, nil
					}
				}
			}
		}

		device, method, err := HandleAutoSelection(ctx, devices, userSession.Username)

		return device, method, err
	}

	return "", "", fmt.Errorf("unknown result: %s", result)
}

// HandleAutoSelection handler automatically selects preferred device if there is only one suitable option.
func HandleAutoSelection(ctx *middlewares.AutheliaCtx, devices []DuoDevice, username string) (string, string, error) {
	if devices == nil {
		ctx.Logger.Debugf("No compatible device/method available for Duo user: %s", username)

		if err := ctx.SetJSONBody(DuoSignResponse{Result: enroll}); err != nil {
			return "", "", fmt.Errorf("unable to set JSON body in response")
		}

		return "", "", nil
	}

	if len(devices) > 1 {
		ctx.Logger.Debugf("Multiple devices available for Duo user: %s - require selection", username)

		if err := ctx.SetJSONBody(DuoSignResponse{Result: auth, Devices: devices}); err != nil {
			return "", "", fmt.Errorf("unable to set JSON body in response")
		}

		return "", "", nil
	}

	if len(devices[0].Capabilities) > 1 {
		ctx.Logger.Debugf("Multiple methods available for Duo user: %s - require selection", username)

		if err := ctx.SetJSONBody(DuoSignResponse{Result: auth, Devices: devices}); err != nil {
			return "", "", fmt.Errorf("unable to set JSON body in response")
		}

		return "", "", nil
	}

	device := devices[0].Device
	method := devices[0].Capabilities[0]
	ctx.Logger.Debugf("Exactly one device: '%s' and method: '%s' found - Saving as new preferred Duo device and method for user: %s", device, method, username)

	if err := ctx.Providers.StorageProvider.SavePreferredDuoDevice(username, device, method); err != nil {
		return "", "", fmt.Errorf("unable to save new preferred Duo device and method for user %s: %s", username, err)
	}

	return device, method, nil
}

// HandleAllow handler for successful logins.
func HandleAllow(ctx *middlewares.AutheliaCtx, targetURL string) {
	userSession := ctx.GetSession()

	err := ctx.Providers.SessionProvider.RegenerateSession(ctx.RequestCtx)
	if err != nil {
		handleAuthenticationUnauthorized(ctx, fmt.Errorf("unable to regenerate session for user %s: %s", userSession.Username, err), messageMFAValidationFailed)
		return
	}

	userSession.SetTwoFactor(ctx.Clock.Now())

	err = ctx.SaveSession(userSession)
	if err != nil {
		handleAuthenticationUnauthorized(ctx, fmt.Errorf("unable to update authentication level with Duo: %s", err), messageMFAValidationFailed)
		return
	}

	if userSession.OIDCWorkflowSession != nil {
		handleOIDCWorkflowResponse(ctx)
	} else {
		Handle2FAResponse(ctx, targetURL)
	}
}

// SetValues sets all appropriate Values for the Auth Request.
func SetValues(userSession session.UserSession, device string, method string, remoteIP string, targetURL string, passcode string) (url.Values, error) {
	values := url.Values{}
	values.Set("username", userSession.Username)
	values.Set("ipaddr", remoteIP)
	values.Set("factor", method)

	switch method {
	case duo.Push:
		values.Set("device", device)

		if userSession.DisplayName != "" {
			values.Set("display_username", userSession.DisplayName)
		}

		if targetURL != "" {
			values.Set("pushinfo", fmt.Sprintf("target%%20url=%s", targetURL))
		}
	case duo.Phone:
		values.Set("device", device)
	case duo.SMS:
		values.Set("device", device)
	case duo.OTP:
		if passcode != "" {
			values.Set("passcode", passcode)
		} else {
			return nil, fmt.Errorf("no passcode received from user: %s", userSession.Username)
		}
	}

	return values, nil
}
