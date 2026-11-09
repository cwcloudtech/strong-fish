import axios from "axios";

const TOKEN_KEY = "sf.token";

/**
 * The API base URL. In development CRA proxies /v1 to the backend (see the
 * package.json "proxy" field), so a blank REACT_APP_APIURL is correct there.
 */
const baseURL = `${(process.env.REACT_APP_APIURL || "").replace(/\/+$/, "")}/v1`;

const client = axios.create({ baseURL });

client.interceptors.request.use((config) => {
  const token = getToken();
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

client.interceptors.response.use(
  (response) => response,
  (error) => {
    // A 401 means the session is gone (expired, or revoked server-side).
    // Clearing it here means every screen reacts the same way rather than each
    // having to notice. The login page itself is exempt: a wrong password
    // there is a 401 too, and reloading would swallow the error message.
    //
    // One 401 must never sign the member out: a connected device whose grant
    // expired. That is about one partner, not the StrongFish session - the API
    // now marks it (424 + errors.deviceReauthorize), and this guard is the
    // belt-and-braces so it can never log the whole account out even if some
    // path still answered 401.
    const isDeviceReauth = error?.response?.data?.i18n_code === "errors.deviceReauthorize";
    if (error?.response?.status === 401 && !isDeviceReauth && !error.config?.url?.includes("/login")) {
      clearToken();
      if (!window.location.pathname.startsWith("/login")) {
        window.location.assign("/login?expired=1");
      }
    }
    return Promise.reject(error);
  }
);

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

export const PAGE_SIZE = Number(process.env.REACT_APP_PAGE_SIZE) || 20;

export default client;
