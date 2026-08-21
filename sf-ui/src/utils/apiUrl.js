/**
 * Where the API lives, as the deployment was configured (SF_API_URL, reaching
 * the browser as REACT_APP_APIURL).
 *
 * Its root serves the Swagger UI, which is the one page in this app that is not
 * part of this app - hence a plain URL rather than a route.
 *
 * A blank setting means "same origin as the frontend", which is the convention
 * the axios client already follows: a deployment that puts the two behind one
 * host has nothing to configure.
 */
export function apiUrl() {
  const configured = (process.env.REACT_APP_APIURL || "").replace(/\/+$/, "");
  return configured || window.location.origin;
}

export default apiUrl;
