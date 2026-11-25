package handlers

import (
	"reflect"

	"strong-fish-api/internal/models"
	"strong-fish-api/internal/openapi"
)

// EndpointDocs maps "METHOD /route" to the request shape the OpenAPI generator
// can't derive from the router alone: the JSON body struct (its schema is
// reflected, so it tracks the struct), and the query parameters the handler
// reads. Only this route->doc mapping is maintained by hand - add a line to
// document another endpoint's body or query params.
//
// It lives in this package (not openapi) because almost every payload is an
// unexported handler type (clubPayload, programMetaPayload, ...) that only this
// package can name; the handful that decode into a models type use that.
//
// The route keys are the exact patterns chi.Walk reports, which is where the
// trailing slashes come from: an index route - Post("/") or Get("/") inside a
// Route(...) subrouter - keeps the slash ("POST /v1/clubs/", "PUT
// /v1/clubs/{clubId}/"), while a named leaf does not ("POST /v1/reports",
// "PUT /v1/clubs/{clubId}/members/{userId}"). Get one wrong and the doc simply
// never attaches. TestValidateEndpointDocs in the router package walks the real
// router and fails on any key here that matches no route, so a mistake is caught
// rather than silently dropped.
func EndpointDocs() map[string]openapi.EndpointDoc {
	t := reflect.TypeOf
	body := func(v any) openapi.EndpointDoc { return openapi.EndpointDoc{Body: t(v)} }
	p := func(name string) openapi.Param { return openapi.Param{Name: name} }
	i := func(name string) openapi.Param { return openapi.Param{Name: name, Type: "integer"} }
	query := func(params ...openapi.Param) openapi.EndpointDoc { return openapi.EndpointDoc{Query: params} }
	// page is the pair the shared pagination() helper reads off every list.
	page := []openapi.Param{i("page"), i("size")}
	// rangeQ is the time window the device data endpoints read off ?range.
	rangeQ := query(p("range"))

	docs := map[string]openapi.EndpointDoc{
		// --- account / session ---
		"POST /v1/users/":                body(registerPayload{}),
		"POST /v1/users/login":           body(credentials{}),
		"POST /v1/users/forgot-password": body(forgotPasswordPayload{}),
		"POST /v1/users/reset-password":  body(resetPasswordPayload{}),
		"PUT /v1/users/me":               body(updateProfilePayload{}),
		"PUT /v1/users/me/picture":       body(updatePicturePayload{}),
		"GET /v1/users/search":           query(p("q")),
		"POST /v1/users/me/config/file":  body(clientConfigPayload{}),
		"POST /v1/users/me/config/qr":    body(clientConfigPayload{}),
		"POST /v1/users/me/api-keys/":    body(apiKeyPayload{}),

		// --- MFA (login challenge + enrollment) ---
		"POST /v1/users/login/mfa/totp":            body(mfaTOTPLoginPayload{}),
		"POST /v1/users/login/mfa/webauthn/begin":  body(mfaChallengeTokenPayload{}),
		"POST /v1/users/login/mfa/webauthn/finish": body(mfaWebAuthnFinishPayload{}),
		"POST /v1/users/me/mfa/totp/confirm":       body(mfaCodePayload{}),
		"POST /v1/users/me/mfa/webauthn/finish":    body(mfaWebAuthnFinishPayload{}),

		// --- clubs / members / invitations ---
		"POST /v1/clubs/":                         body(clubPayload{}),
		"PUT /v1/clubs/{clubId}/":                 body(clubPayload{}),
		"POST /v1/clubs/{clubId}/transfer":        body(transferOwnershipPayload{}),
		"POST /v1/clubs/{clubId}/members/":        body(addMemberPayload{}),
		"PUT /v1/clubs/{clubId}/members/{userId}": body(memberRolePayload{}),
		"POST /v1/clubs/{clubId}/invitations/":    body(invitePayload{}),
		"GET /v1/clubs/{clubId}/feed":             {Query: page},
		"GET /v1/clubs/{clubId}/feedback":         {Query: page},

		// --- exercises / one-rms ---
		"GET /v1/exercises/":             query(p("q")),
		"POST /v1/exercises/":            body(exercisePayload{}),
		"PUT /v1/exercises/{exerciseId}": body(exercisePayload{}),
		"PUT /v1/one-rms/{exerciseId}":   body(oneRMPayload{}),

		// --- clubs: the one program list that filters by member ---
		"GET /v1/clubs/{clubId}/programs/": query(p("memberId")),

		// --- training (the member's assigned blocks) ---
		"PUT /v1/training/{assignmentId}/status":           body(assignmentStatusPayload{}),
		"PUT /v1/training/{assignmentId}/sets/{setId}/log": body(logSetPayload{}),
		"PUT /v1/training/{assignmentId}/sets/{setId}/vbt": body(models.BarSetVBT{}),
		"PUT /v1/training/{assignmentId}/days/{dayId}/log": body(dayDonePayload{}),

		// --- social feed / posts / comments / reports ---
		"GET /v1/posts/":                              {Query: page},
		"GET /v1/posts/discover":                      {Query: page},
		"POST /v1/posts/":                             body(postPayload{}),
		"PUT /v1/posts/{postId}/":                     body(postPayload{}),
		"GET /v1/posts/{postId}/comments/":            {Query: page},
		"POST /v1/posts/{postId}/comments/":           body(commentPayload{}),
		"PUT /v1/posts/{postId}/comments/{commentId}": body(commentPayload{}),
		"POST /v1/reports":                            body(reportPayload{}),

		// --- profiles / search ---
		"GET /v1/profiles/{handle}/posts":   {Query: page},
		"GET /v1/profiles/{handle}/follows": query(p("direction")),
		"GET /v1/search/members": {Query: append(
			[]openapi.Param{p("terms"), p("name"), p("surname"), p("username"), p("email")}, page...)},

		// --- messages ---
		"POST /v1/messages/with/{userId}": body(messagePayload{}),

		// --- events (calendar) ---
		"GET /v1/events":           query(p("from"), p("past")),
		"POST /v1/events":          body(eventPayload{}),
		"PUT /v1/events/{eventId}": body(eventPayload{}),

		// --- strength / bar speed ---
		"POST /v1/strength/score":             body(scorePayload{}),
		"POST /v1/users/me/bar-speed/analyze": body(analyzePayload{}),
		"POST /v1/users/me/bar-speed/effort":  body(effortPayload{}),

		// --- connected devices: pairing + the time-windowed reads ---
		"POST /v1/users/me/devices/{provider}/pair":            body(pairPayload{}),
		"POST /v1/users/me/devices/{provider}/google":          body(googlePairPayload{}),
		"POST /v1/users/me/devices/{provider}/pair-key":        body(keyPairPayload{}),
		"GET /v1/users/me/devices/{provider}/steps":            rangeQ,
		"GET /v1/users/me/devices/{provider}/heart-rate":       rangeQ,
		"GET /v1/users/me/devices/{provider}/body-composition": rangeQ,
		"GET /v1/users/me/devices/{provider}/sleep":            rangeQ,
		"GET /v1/users/me/devices/{provider}/nutrition":        rangeQ,

		// --- the member's own storages (video buckets) ---
		"POST /v1/users/me/storages/":                   body(models.StorageConnection{}),
		"PUT /v1/users/me/storages/order":               body(storageOrderPayload{}),
		"PUT /v1/users/me/storages/{storageId}/":        body(models.StorageConnection{}),
		"POST /v1/users/me/storages/{storageId}/shares": body(storageGrantPayload{}),

		// --- the member's own model connections ---
		"POST /v1/users/me/ai-connections/":                      body(aiConnectionPayload{}),
		"PUT /v1/users/me/ai-connections/order":                  body(aiOrderPayload{}),
		"PUT /v1/users/me/ai-connections/{connectionId}/":        body(aiConnectionPayload{}),
		"POST /v1/users/me/ai-connections/{connectionId}/shares": body(aiGrantPayload{}),

		// --- the member's own recording callbacks ---
		"POST /v1/users/me/callbacks/":             body(callbackPayload{}),
		"PUT /v1/users/me/callbacks/{callbackId}/": body(callbackPayload{}),

		// --- AI assistant (chat + conversations) ---
		"POST /v1/ai/chat":                           body(chatPayload{}),
		"POST /v1/ai/cwcloud/adapter":                body(cwAdapterPayload{}),
		"POST /v1/ai/conversations/delete":           body(deleteConversationsPayload{}),
		"PUT /v1/ai/conversations/{conversationId}/": body(conversationTitlePayload{}),

		// --- contact form (public) ---
		"POST /v1/contact": body(contactPayload{}),

		// --- superadmin ---
		"GET /v1/admin/ips/":                    query(p("term"), i("page")),
		"PUT /v1/admin/ips/":                    body(models.IpBanAction{}),
		"PUT /v1/admin/users/{userId}":          body(adminUserPayload{}),
		"PUT /v1/admin/coach-requests/{userId}": body(coachDecisionPayload{}),
		"GET /v1/admin/reports":                 {Query: append([]openapi.Param{p("status")}, page...)},
		"PUT /v1/admin/reports/{reportId}":      body(resolveReportPayload{}),
	}

	// The program editing endpoints are mounted twice - once under a club and
	// once under the member's own library - with the same handlers and the same
	// request bodies, so document both trees from one list rather than keeping
	// two copies in step by hand. The index routes (the list root and the
	// {programId} root) keep chi's trailing slash; the named sub-paths don't.
	programWrites := func(base string) {
		docs["POST "+base+"/"] = body(programMetaPayload{})
		docs["PUT "+base+"/{programId}/"] = body(programMetaPayload{})
		docs["POST "+base+"/{programId}/days"] = body(dayPayload{})
		docs["PUT "+base+"/{programId}/days/{dayId}"] = body(dayPayload{})
		docs["PUT "+base+"/{programId}/days/{dayId}/move"] = body(moveDayPayload{})
		docs["POST "+base+"/{programId}/days/{dayId}/sets"] = body(setPayload{})
		docs["PUT "+base+"/{programId}/sets/{setId}"] = body(setPayload{})
		docs["PUT "+base+"/{programId}/sets/{setId}/move"] = body(moveSetPayload{})
		docs["POST "+base+"/{programId}/assignments"] = body(assignPayload{})
		docs["POST "+base+"/{programId}/copy"] = body(copyProgramPayload{})
	}
	programWrites("/v1/clubs/{clubId}/programs")
	programWrites("/v1/programs")

	return docs
}
