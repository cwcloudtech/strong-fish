package handlers

import (
	"net/http"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/models"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/utils"
)

// SearchHandler finds members by email, name or surname.
//
// It is shaped like uprodit's own search - one query parameter per criterion,
// combined with AND, plus a free-text `terms` that matches any of them - so a
// caller can narrow by exactly what they know rather than guessing at a single
// box. Paging is the API's usual page/size.
//
// What comes back depends on who is asking: the visibility rules run inside the
// query (see UserStore.SearchMembers), so a profile the caller may not see is
// not merely hidden from the results, it never counts towards them.
type SearchHandler struct {
	users   *store.UserStore
	clubs   *store.ClubStore
	profile *ProfileHandler
}

func NewSearchHandler(users *store.UserStore, clubs *store.ClubStore, profile *ProfileHandler) *SearchHandler {
	return &SearchHandler{users: users, clubs: clubs, profile: profile}
}

// searchResult is a member as a result row shows them: enough to recognize
// somebody and open their profile, and nothing more. The email is deliberately
// absent even though it can be searched on - being able to confirm an address
// you already know is not the same as being handed everybody's.
type searchResult struct {
	ID       string            `json:"id"`
	Handle   string            `json:"handle,omitempty"`
	Name     string            `json:"name"`
	Surname  string            `json:"surname"`
	Role     models.GlobalRole `json:"role"`
	Picture  string            `json:"picture,omitempty"`
	PictureX float64           `json:"pictureX"`
	PictureY float64           `json:"pictureY"`
	// SharesClub tells the caller why this person is visible to them, which is
	// what makes a result list of near-identical names usable.
	SharesClub bool `json:"sharesClub"`
	// Anonymous says the name above is a username rather than a person's name.
	Anonymous bool `json:"anonymous,omitempty"`
}

func (h *SearchHandler) Members(w http.ResponseWriter, r *http.Request) {
	callerID, _ := middleware.UserIDFromContext(r.Context())

	superadmin := false
	if utils.IsNotBlank(callerID) {
		if caller, err := h.users.FindByID(r.Context(), callerID); err == nil {
			superadmin = caller.Role == models.GlobalRoleSuperadmin
		}
	}

	page, size := pagination(r)
	query := r.URL.Query()
	users, total, err := h.users.SearchMembers(r.Context(), store.MemberSearch{
		Terms:   query.Get("terms"),
		Name:    query.Get("name"),
		Surname: query.Get("surname"),
		Email:   query.Get("email"),
		Page:    page,
		Size:    size,
	}, callerID, superadmin)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	// One query for the whole page rather than one per row: the club-mate set
	// is the same for every result, and the alternative is N round trips to
	// answer the same question.
	mates := map[string]bool{}
	if utils.IsNotBlank(callerID) {
		if ids, err := h.clubs.ListClubMateIDs(r.Context(), callerID); err == nil {
			for _, id := range ids {
				mates[id] = true
			}
		}
	}

	results := make([]searchResult, len(users))
	for i, user := range users {
		// Projected through DisplayName, not read off the record: the store
		// returns the true account, and it is every projection's job to honour
		// the member's choice to be known by their username.
		name, surname := user.DisplayName()
		results[i] = searchResult{
			ID: user.ID, Handle: user.Handle, Name: name, Surname: surname,
			Role: user.Role, Picture: user.Picture, PictureX: user.PictureX,
			PictureY: user.PictureY, SharesClub: mates[user.ID], Anonymous: user.Anonymous,
		}
	}
	writeJSON(w, http.StatusOK, models.Page[searchResult]{Results: results, TotalResults: total})
}
