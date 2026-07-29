package v2

import "github.com/gin-gonic/gin"

// ProjectScoper is the authorization boundary for the v2 read surface.
//
// The test-run and trends handlers consult it to constrain (or reject) a
// request's project filter so a non-admin cannot read another team's data.
// v1 enforces the same rule inside its GraphQL resolvers; the v2 REST
// handlers previously enforced only authentication, which let any signed-in
// user query any project's runs by passing its id (see the migration guide
// and the fix in the same PR).
//
// Implementations live at the wiring layer (cmd/fern-platform), where the
// auth and project services are available. The handlers depend only on this
// interface so they stay unit-testable with a fake. A nil ProjectScoper
// disables enforcement — appropriate only for local AUTH_ENABLED=false runs,
// where the injected user is a synthetic admin anyway.
type ProjectScoper interface {
	// AccessibleProjects returns the set of project ids the caller may
	// read. unrestricted=true means the caller is an admin and no filtering
	// should be applied. A non-nil error means the caller could not be
	// identified or the project list failed; the handler treats it as an
	// internal error and returns no data (fail closed).
	AccessibleProjects(c *gin.Context) (allowed map[string]struct{}, unrestricted bool, err error)
}

// scopeProjectIDs constrains a requested project-id list to the allowed set.
//
// When requested is empty the caller asked for "everything I can see", so
// the full allowed set is returned. Otherwise the result is the
// intersection requested ∩ allowed. ok is false when the caller ends up
// with zero accessible projects: the handler must then return an empty
// result rather than run an unfiltered query, which would leak every
// team's data.
func scopeProjectIDs(requested []string, allowed map[string]struct{}) (ids []string, ok bool) {
	if len(requested) == 0 {
		out := make([]string, 0, len(allowed))
		for id := range allowed {
			out = append(out, id)
		}
		return out, len(out) > 0
	}
	out := make([]string, 0, len(requested))
	for _, id := range requested {
		if _, allow := allowed[id]; allow {
			out = append(out, id)
		}
	}
	return out, len(out) > 0
}

// allowedIDs flattens the accessible-project set into a slice for the
// filter's authorization boundary.
func allowedIDs(allowed map[string]struct{}) []string {
	out := make([]string, 0, len(allowed))
	for id := range allowed {
		out = append(out, id)
	}
	return out
}
