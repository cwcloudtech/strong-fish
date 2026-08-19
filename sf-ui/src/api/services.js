import client, { PAGE_SIZE } from "./client";

/**
 * One module per resource would be six files of three lines each; grouping them
 * by domain keeps the API surface readable in one place. Every function returns
 * the response body directly - callers care about data, not envelopes.
 */

const body = (promise) => promise.then((response) => response.data);

// --- auth & account ---

export const auth = {
  register: (payload) => body(client.post("/users", payload)),
  login: (email, password) => body(client.post("/users/login", { email, password })),
  me: () => body(client.get("/users/me")),
  updateProfile: (payload) => body(client.put("/users/me", payload)),
  updatePicture: (picture, x, y) => body(client.put("/users/me/picture", { picture, x, y })),
  forgotPassword: (email) => body(client.post("/users/forgot-password", { email })),
  resetPassword: (payload) => body(client.post("/users/reset-password", payload)),
  search: (q) => body(client.get("/users/search", { params: { q } })),
  config: () => body(client.get("/config")),
};

export const mfa = {
  status: () => body(client.get("/users/me/mfa")),
  totpSetup: () => body(client.post("/users/me/mfa/totp/setup")),
  totpConfirm: (code) => body(client.post("/users/me/mfa/totp/confirm", { code })),
  totpDisable: () => body(client.delete("/users/me/mfa/totp")),
  webauthnRegisterBegin: () => body(client.post("/users/me/mfa/webauthn/begin")),
  webauthnRegisterFinish: (payload) => body(client.post("/users/me/mfa/webauthn/finish", payload)),
  webauthnDelete: (credentialId) => body(client.delete(`/users/me/mfa/webauthn/${credentialId}`)),
  loginTotp: (challengeToken, code) => body(client.post("/users/login/mfa/totp", { challengeToken, code })),
  loginWebauthnBegin: (challengeToken) => body(client.post("/users/login/mfa/webauthn/begin", { challengeToken })),
  loginWebauthnFinish: (payload) => body(client.post("/users/login/mfa/webauthn/finish", payload)),
};

// --- clubs ---

export const clubs = {
  list: () => body(client.get("/clubs")),
  listAll: () => body(client.get("/admin/clubs")),
  create: (payload) => body(client.post("/clubs", payload)),
  get: (clubId) => body(client.get(`/clubs/${clubId}`)),
  update: (clubId, payload) => body(client.put(`/clubs/${clubId}`, payload)),
  remove: (clubId) => body(client.delete(`/clubs/${clubId}`)),
  members: (clubId) => body(client.get(`/clubs/${clubId}/members`)),
  addMember: (clubId, payload) => body(client.post(`/clubs/${clubId}/members`, payload)),
  setMemberRole: (clubId, userId, role) => body(client.put(`/clubs/${clubId}/members/${userId}`, { role })),
  removeMember: (clubId, userId) => body(client.delete(`/clubs/${clubId}/members/${userId}`)),
  leave: (clubId) => body(client.delete(`/clubs/${clubId}/members/me`)),
  transfer: (clubId, userId) => body(client.post(`/clubs/${clubId}/transfer`, { userId })),
  feed: (clubId, page = 0) => body(client.get(`/clubs/${clubId}/feed`, { params: { page, size: PAGE_SIZE } })),
  feedback: (clubId, page = 0) => body(client.get(`/clubs/${clubId}/feedback`, { params: { page, size: PAGE_SIZE } })),
};

// --- exercises & 1RMs ---

export const exercises = {
  list: (q) => body(client.get("/exercises", { params: q ? { q } : {} })),
  create: (payload) => body(client.post("/exercises", payload)),
  update: (exerciseId, payload) => body(client.put(`/exercises/${exerciseId}`, payload)),
  // What a delete would take with it, so the superadmin confirms an informed
  // cascade rather than discovering it afterwards.
  usage: (exerciseId) => body(client.get(`/exercises/${exerciseId}/usage`)),
  remove: (exerciseId) => body(client.delete(`/exercises/${exerciseId}`)),
};

export const oneRms = {
  list: () => body(client.get("/one-rms")),
  set: (exerciseId, value) => body(client.put(`/one-rms/${exerciseId}`, { value })),
  remove: (exerciseId) => body(client.delete(`/one-rms/${exerciseId}`)),
};

// --- programs ---

export const programs = {
  list: (clubId) => body(client.get(`/clubs/${clubId}/programs`)),
  create: (clubId, payload) => body(client.post(`/clubs/${clubId}/programs`, payload)),
  get: (clubId, programId, memberId) =>
    body(client.get(`/clubs/${clubId}/programs/${programId}`, { params: memberId ? { memberId } : {} })),
  update: (clubId, programId, payload) => body(client.put(`/clubs/${clubId}/programs/${programId}`, payload)),
  remove: (clubId, programId) => body(client.delete(`/clubs/${clubId}/programs/${programId}`)),
  importFile: (clubId, file, name, description) => {
    const form = new FormData();
    form.append("file", file);
    if (name) form.append("name", name);
    if (description) form.append("description", description);
    return body(client.post(`/clubs/${clubId}/programs/import`, form));
  },
  addDay: (clubId, programId, payload) =>
    body(client.post(`/clubs/${clubId}/programs/${programId}/days`, payload)),
  updateDay: (clubId, programId, dayId, payload) =>
    body(client.put(`/clubs/${clubId}/programs/${programId}/days/${dayId}`, payload)),
  removeDay: (clubId, programId, dayId) =>
    body(client.delete(`/clubs/${clubId}/programs/${programId}/days/${dayId}`)),
  addSet: (clubId, programId, dayId, payload) =>
    body(client.post(`/clubs/${clubId}/programs/${programId}/days/${dayId}/sets`, payload)),
  updateSet: (clubId, programId, setId, payload) =>
    body(client.put(`/clubs/${clubId}/programs/${programId}/sets/${setId}`, payload)),
  removeSet: (clubId, programId, setId) =>
    body(client.delete(`/clubs/${clubId}/programs/${programId}/sets/${setId}`)),
  assignments: (clubId, programId) => body(client.get(`/clubs/${clubId}/programs/${programId}/assignments`)),
  assign: (clubId, programId, payload) =>
    body(client.post(`/clubs/${clubId}/programs/${programId}/assignments`, payload)),
  unassign: (clubId, programId, assignmentId) =>
    body(client.delete(`/clubs/${clubId}/programs/${programId}/assignments/${assignmentId}`)),
};

// --- the member's own training ---

export const training = {
  list: () => body(client.get("/training")),
  get: (assignmentId) => body(client.get(`/training/${assignmentId}`)),
  setStatus: (assignmentId, status) => body(client.put(`/training/${assignmentId}/status`, { status })),
  logSet: (assignmentId, setId, payload) => body(client.put(`/training/${assignmentId}/sets/${setId}/log`, payload)),
  clearLog: (assignmentId, setId) => body(client.delete(`/training/${assignmentId}/sets/${setId}/log`)),
};

// --- social ---

export const social = {
  feed: (page = 0) => body(client.get("/posts", { params: { page, size: PAGE_SIZE } })),
  discover: (page = 0) => body(client.get("/posts/discover", { params: { page, size: PAGE_SIZE } })),
  createPost: (payload) => body(client.post("/posts", payload)),
  updatePost: (postId, payload) => body(client.put(`/posts/${postId}`, payload)),
  removePost: (postId) => body(client.delete(`/posts/${postId}`)),
  like: (postId) => body(client.post(`/posts/${postId}/like`)),
  unlike: (postId) => body(client.delete(`/posts/${postId}/like`)),
  comments: (postId, page = 0) => body(client.get(`/posts/${postId}/comments`, { params: { page, size: PAGE_SIZE } })),
  addComment: (postId, content) => body(client.post(`/posts/${postId}/comments`, { content })),
  updateComment: (postId, commentId, content) =>
    body(client.put(`/posts/${postId}/comments/${commentId}`, { content })),
  removeComment: (postId, commentId) => body(client.delete(`/posts/${postId}/comments/${commentId}`)),
  report: (payload) => body(client.post("/reports", payload)),
};

export const profiles = {
  get: (handle) => body(client.get(`/profiles/${handle}`)),
  posts: (handle, page = 0) => body(client.get(`/profiles/${handle}/posts`, { params: { page, size: PAGE_SIZE } })),
  follows: (handle, direction) => body(client.get(`/profiles/${handle}/follows`, { params: { direction } })),
  follow: (handle) => body(client.post(`/profiles/${handle}/follow`)),
  unfollow: (handle) => body(client.delete(`/profiles/${handle}/follow`)),
};

// --- search and invitations ---

export const search = {
  // Readable logged out, but a logged-out caller only ever gets public
  // profiles: the visibility rules run inside the API's query.
  members: (params) => body(client.get("/search/members", { params })),
};

export const invitations = {
  mine: () => body(client.get("/users/me/invitations")),
  accept: (invitationId) => body(client.post(`/users/me/invitations/${invitationId}/accept`)),
  decline: (invitationId) => body(client.post(`/users/me/invitations/${invitationId}/decline`)),
  forClub: (clubId) => body(client.get(`/clubs/${clubId}/invitations`)),
  invite: (clubId, payload) => body(client.post(`/clubs/${clubId}/invitations`, payload)),
  withdraw: (clubId, invitationId) => body(client.delete(`/clubs/${clubId}/invitations/${invitationId}`)),
};

// --- private messages and the block list ---

export const messages = {
  conversations: () => body(client.get("/messages")),
  unread: () => body(client.get("/messages/unread")),
  // Addressed by who, not by conversation id: there is exactly one thread per
  // pair, so the id is the API's business rather than the client's.
  thread: (userId) => body(client.get(`/messages/with/${userId}`)),
  // The payload carries text, pictures and a voice message's URL - the API
  // derives the link from the text, as it does for a post.
  send: (userId, payload) => body(client.post(`/messages/with/${userId}`, payload)),
};

export const blocks = {
  list: () => body(client.get("/blocks")),
  block: (userId) => body(client.post(`/blocks/${userId}`)),
  unblock: (userId) => body(client.delete(`/blocks/${userId}`)),
};

// --- media, events and the calendar feed ---

export const media = {
  // Uploads to the member's *own* bucket and returns its public URL. 405 means
  // they haven't configured one, which is what the composer toasts.
  uploadVideo: (file, onProgress) => {
    const form = new FormData();
    form.append("file", file);
    return body(
      client.post("/media/videos", form, {
        onUploadProgress: (event) =>
          onProgress?.(event.total ? Math.round((event.loaded * 100) / event.total) : 0),
      })
    );
  },
  // Same storage, same 405 when none is configured. Kept separate from the
  // video upload because the accepted types and the size cap differ.
  uploadAudio: (blob, filename) => {
    const form = new FormData();
    form.append("file", blob, filename);
    return body(client.post("/media/audio", form));
  },
  storage: () => body(client.get("/users/me/storage")),
  setStorage: (payload) => body(client.put("/users/me/storage", payload)),
  clearStorage: () => body(client.delete("/users/me/storage")),
};

export const events = {
  // Readable logged out; what comes back still widens for a member, whose own
  // clubs' dates join the public ones.
  list: (params) => body(client.get("/events", { params })),
  get: (eventId) => body(client.get(`/events/${eventId}`)),
  create: (payload) => body(client.post("/events", payload)),
  update: (eventId, payload) => body(client.put(`/events/${eventId}`, payload)),
  remove: (eventId) => body(client.delete(`/events/${eventId}`)),
};

export const calendarFeed = {
  status: () => body(client.get("/users/me/calendar-feed")),
  enable: () => body(client.post("/users/me/calendar-feed/enable")),
  disable: () => body(client.post("/users/me/calendar-feed/disable")),
  regenerate: () => body(client.post("/users/me/calendar-feed/regenerate")),
};

// --- API keys ---

export const apiKeys = {
  list: () => body(client.get("/users/me/api-keys")),
  // The response to this call is the only place the plaintext token ever
  // exists - the API stores its hash and nothing else - so whatever the caller
  // means to do with it, it has to do now.
  create: (payload) => body(client.post("/users/me/api-keys", payload)),
  remove: (keyId) => body(client.delete(`/users/me/api-keys/${keyId}`)),
  // The token goes back to the API to be formatted, because the API is what
  // knows its own public URL. It is POSTed rather than sent as a header so no
  // reverse proxy in front of this needs a CORS exception for it.
  configQr: (key) => body(client.post("/users/me/config/qr", { key })),
  configFile: (key) => client.post("/users/me/config/file", { key }, { responseType: "blob" }).then((r) => r.data),
};

// --- the mobile build ---

export const mobileApp = {
  // Public: you need the app before you have an account.
  get: () => body(client.get("/mobile-app")),
};

// --- programs shared publicly ---

export const publicPosts = {
  // Readable with no account, and only ever a post published to everybody -
  // the API's own visibility predicate decides, not this call.
  get: (postId) => body(client.get(`/public/posts/${postId}`)),
};

export const publicPrograms = {
  get: (programId) => body(client.get(`/public/programs/${programId}`)),
};

// --- contact ---

export const contact = {
  // Forwarded by the API to CWCloud's contact-request endpoint; name and
  // firstname are optional.
  send: (payload) => body(client.post("/contact", payload)),
};

// --- administration ---

export const admin = {
  stats: () => body(client.get("/admin/stats")),
  users: () => body(client.get("/admin/users")),
  updateUser: (userId, payload) => body(client.put(`/admin/users/${userId}`, payload)),
  removeUser: (userId) => body(client.delete(`/admin/users/${userId}`)),
  clearMfa: (userId) => body(client.delete(`/admin/users/${userId}/mfa`)),
  userIps: (userId) => body(client.get(`/admin/users/${userId}/ips`)),
  coachRequests: () => body(client.get("/admin/coach-requests")),
  decideCoachRequest: (userId, status, motive) =>
    body(client.put(`/admin/coach-requests/${userId}`, { status, motive })),
  reports: (status, page = 0) =>
    body(client.get("/admin/reports", { params: { status: status || undefined, page, size: PAGE_SIZE } })),
  resolveReport: (reportId, status, deleteTarget) =>
    body(client.put(`/admin/reports/${reportId}`, { status, deleteTarget })),
};
