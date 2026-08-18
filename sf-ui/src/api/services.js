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
  reports: (status, page = 0) =>
    body(client.get("/admin/reports", { params: { status: status || undefined, page, size: PAGE_SIZE } })),
  resolveReport: (reportId, status, deleteTarget) =>
    body(client.put(`/admin/reports/${reportId}`, { status, deleteTarget })),
};
