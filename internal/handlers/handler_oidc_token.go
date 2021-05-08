package handlers

import (
	"net/http"

	"github.com/ory/fosite"

	"github.com/authelia/authelia/internal/middlewares"
)

func oidcToken(ctx *middlewares.AutheliaCtx, rw http.ResponseWriter, req *http.Request) {
	oidcSession := newOpenIDSession("")

	accessRequest, accessReqErr := ctx.Providers.OpenIDConnect.Fosite.NewAccessRequest(ctx, req, oidcSession)
	if accessReqErr != nil {
		ctx.Logger.Errorf("Error occurred in NewAccessRequest: %+v", accessRequest)
		ctx.Providers.OpenIDConnect.Fosite.WriteAccessError(rw, accessRequest, accessReqErr)

		return
	}

	// If this is a client_credentials grant, grant all scopes the client is allowed to perform.
	if accessRequest.GetGrantTypes().ExactOne("client_credentials") {
		for _, scope := range accessRequest.GetRequestedScopes() {
			if fosite.HierarchicScopeStrategy(accessRequest.GetClient().GetScopes(), scope) {
				accessRequest.GrantScope(scope)
			}
		}
	}

	response, err := ctx.Providers.OpenIDConnect.Fosite.NewAccessResponse(ctx, accessRequest)
	if err != nil {
		ctx.Logger.Errorf("Error occurred in NewAccessResponse: %+v", err)
		ctx.Providers.OpenIDConnect.Fosite.WriteAccessError(rw, accessRequest, err)

		return
	}

	ctx.Providers.OpenIDConnect.Fosite.WriteAccessResponse(rw, accessRequest, response)
}
