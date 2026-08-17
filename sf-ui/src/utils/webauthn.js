/**
 * WebAuthn plumbing for security-key MFA (a YubiKey, or the device's built-in
 * authenticator).
 *
 * The browser API works in ArrayBuffers while JSON only carries strings, so
 * every challenge/id field has to be converted on the way in and back on the way
 * out. The server speaks base64url (what the WebAuthn spec and the go-webauthn
 * library both use), so that's what these helpers translate.
 */

export function isWebAuthnSupported() {
  return typeof window !== "undefined" && Boolean(window.PublicKeyCredential);
}

function base64urlToBuffer(value) {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(padded + "=".repeat((4 - (padded.length % 4)) % 4));
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
  return bytes.buffer;
}

function bufferToBase64url(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.length; i += 1) binary += String.fromCharCode(bytes[i]);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

/** Decodes the credential-creation options the server sent. */
function decodeCreationOptions(options) {
  const publicKey = { ...options.publicKey };
  publicKey.challenge = base64urlToBuffer(publicKey.challenge);
  publicKey.user = { ...publicKey.user, id: base64urlToBuffer(publicKey.user.id) };
  if (publicKey.excludeCredentials) {
    publicKey.excludeCredentials = publicKey.excludeCredentials.map((credential) => ({
      ...credential,
      id: base64urlToBuffer(credential.id),
    }));
  }
  return publicKey;
}

/** Decodes the assertion options the server sent. */
function decodeRequestOptions(options) {
  const publicKey = { ...options.publicKey };
  publicKey.challenge = base64urlToBuffer(publicKey.challenge);
  if (publicKey.allowCredentials) {
    publicKey.allowCredentials = publicKey.allowCredentials.map((credential) => ({
      ...credential,
      id: base64urlToBuffer(credential.id),
    }));
  }
  return publicKey;
}

/** Registers a new security key and returns the attestation to send back. */
export async function createCredential(options) {
  const credential = await navigator.credentials.create({ publicKey: decodeCreationOptions(options) });
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    // Some authenticators don't report their transports; an empty list is valid.
    transports: credential.response.getTransports?.() || [],
    response: {
      clientDataJSON: bufferToBase64url(credential.response.clientDataJSON),
      attestationObject: bufferToBase64url(credential.response.attestationObject),
    },
  };
}

/** Signs a login challenge with a registered key. */
export async function getAssertion(options) {
  const credential = await navigator.credentials.get({ publicKey: decodeRequestOptions(options) });
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: bufferToBase64url(credential.response.clientDataJSON),
      authenticatorData: bufferToBase64url(credential.response.authenticatorData),
      signature: bufferToBase64url(credential.response.signature),
      userHandle: credential.response.userHandle ? bufferToBase64url(credential.response.userHandle) : null,
    },
  };
}
