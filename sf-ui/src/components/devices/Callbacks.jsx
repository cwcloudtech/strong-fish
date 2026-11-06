import { useCallback, useEffect, useState } from "react";
import { toast } from "react-toastify";
import { FiEdit2, FiPlus, FiTrash2 } from "react-icons/fi";

import Select from "../common/Select";
import Switch from "../common/Switch";
import toastOptions from "../../utils/toastOptions";
import { callbacks as callbacksApi } from "../../api/services";
import useToastError from "../../utils/useToast";
import { useI18n } from "../../i18n/I18nContext";

const TYPES = ["http", "websocket", "mqtt"];

// A placeholder URL that reads as the right shape for each transport.
const URL_HINTS = {
  http: "http://esp32.local/record",
  websocket: "ws://esp32.local/ws",
  mqtt: "mqtt://broker.local:1883",
};

/**
 * The member's recording callbacks: triggers the phone fires when they start a
 * bar-speed set, so a device of their own - an ESP32 on the gym network -
 * records the same set, with the lift and the load in the body.
 *
 * A webhook (HTTP POST) is one transport; a WebSocket message and an MQTT
 * publish are the others - cwcloud's "callback", where the body is the same
 * whichever way it travels. Fired from the phone, since the destination is
 * usually on the local network only the phone can reach.
 */
export default function Callbacks() {
  const { t } = useI18n();
  const toastError = useToastError();
  const [list, setList] = useState(null);
  const [busy, setBusy] = useState(false);
  // Which callback's form is open: an id, "new" while adding one, or null.
  const [editing, setEditing] = useState(null);

  const load = useCallback(async () => {
    try {
      setList(await callbacksApi.list());
    } catch {
      setList([]);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (!list) return null;

  const act = async (run, message) => {
    setBusy(true);
    try {
      await run();
      await load();
      if (message) toast.success(message, toastOptions);
      setEditing(null);
    } catch (err) {
      toastError(err);
    } finally {
      setBusy(false);
    }
  };

  // Flipping the switch is an update that keeps the rest. Blank secrets keep the
  // stored ones, so toggling a device on and off never loses its credentials.
  const toggle = (callback) =>
    act(() =>
      callbacksApi.set(callback.id, {
        name: callback.name,
        type: callback.type,
        url: callback.url,
        headers: callback.headers || [],
        username: callback.username || "",
        topic: callback.topic || "",
        clientId: callback.clientId || "",
        qos: callback.qos || 0,
        token: "",
        password: "",
        enabled: !callback.enabled,
      }),
    );

  return (
    <div className="sf-card">
      <h2>{t("callbacks.title")}</h2>
      <p className="sf-subtitle">{t("callbacks.subtitle")}</p>

      {list.length === 0 ? (
        <p className="sf-muted">{t("callbacks.none")}</p>
      ) : (
        <ul className="sf-list">
          {list.map((callback) => (
            <li className="sf-list-item" key={callback.id} style={{ flexWrap: "wrap" }}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>{callback.name || callback.url}</strong>
                <div className="sf-muted" style={{ fontSize: "0.85rem", wordBreak: "break-all" }}>
                  {[
                    t(`callbacks.type.${callback.type}`),
                    callback.url,
                    callback.enabled ? null : t("callbacks.disabled"),
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </div>
              </div>

              <div className="sf-row" style={{ gap: "0.4rem", alignItems: "center" }}>
                <Switch
                  checked={callback.enabled}
                  onChange={() => toggle(callback)}
                  disabled={busy}
                  aria-label={t("callbacks.enabled")}
                  title={t("callbacks.enabled")}
                />
                <button
                  type="button"
                  className="sf-button-ghost sf-button-sm"
                  onClick={() => setEditing(editing === callback.id ? null : callback.id)}
                  aria-label={t("common.edit")}
                  title={t("common.edit")}
                >
                  <FiEdit2 />
                </button>
                <button
                  type="button"
                  className="sf-button-ghost sf-button-sm"
                  onClick={() => act(() => callbacksApi.remove(callback.id), t("callbacks.removed"))}
                  disabled={busy}
                  aria-label={t("common.delete")}
                  title={t("common.delete")}
                >
                  <FiTrash2 />
                </button>
              </div>

              {editing === callback.id ? (
                <div style={{ flexBasis: "100%" }}>
                  <CallbackForm
                    callback={callback}
                    busy={busy}
                    onCancel={() => setEditing(null)}
                    onSubmit={(payload) => act(() => callbacksApi.set(callback.id, payload), t("callbacks.saved"))}
                  />
                </div>
              ) : null}
            </li>
          ))}
        </ul>
      )}

      {editing === "new" ? (
        <CallbackForm
          callback={null}
          busy={busy}
          onCancel={() => setEditing(null)}
          onSubmit={(payload) => act(() => callbacksApi.add(payload), t("callbacks.saved"))}
        />
      ) : (
        <button type="button" className="sf-button" onClick={() => setEditing("new")}>
          <FiPlus /> {t("callbacks.add")}
        </button>
      )}
    </div>
  );
}

/**
 * The form for a new callback or one being edited: the transport, the URL, and
 * the credentials each kind needs.
 *
 * Headers are edited as "Name: value" lines, one per header - less to click
 * through than a row of inputs, and the member setting up an ESP32 is reading a
 * header name off a datasheet anyway. The two secrets (token, password) are
 * write-only: left blank on an edit, the server keeps the stored one.
 */
function CallbackForm({ callback, busy, onCancel, onSubmit }) {
  const { t } = useI18n();
  const [form, setForm] = useState({
    name: callback?.name || "",
    type: callback?.type || "http",
    url: callback?.url || "",
    token: "",
    headersText: headersToText(callback?.headers),
    username: callback?.username || "",
    password: "",
    topic: callback?.topic || "",
    clientId: callback?.clientId || "",
    qos: callback?.qos ? String(callback.qos) : "0",
    enabled: callback ? callback.enabled : true,
  });

  const set = (field) => (event) => setForm((current) => ({ ...current, [field]: event.target.value }));

  const submit = (event) => {
    event.preventDefault();
    onSubmit({
      name: form.name.trim(),
      type: form.type,
      url: form.url.trim(),
      token: form.token.trim(),
      headers: textToHeaders(form.headersText),
      username: form.username.trim(),
      password: form.password,
      topic: form.topic.trim(),
      clientId: form.clientId.trim(),
      qos: Number.parseInt(form.qos, 10) || 0,
      enabled: form.enabled,
    });
  };

  const isHttp = form.type === "http";
  const isMqtt = form.type === "mqtt";
  // Headers and a bearer token ride on the request; both the webhook and the
  // WebSocket carry them, MQTT does not.
  const carriesHeaders = form.type === "http" || form.type === "websocket";

  return (
    <form className="sf-card" style={{ marginTop: "0.8rem" }} onSubmit={submit}>
      <div className="sf-row" style={{ gap: "0.6rem", flexWrap: "wrap" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("callbacks.name")}</label>
          <input
            className="sf-input"
            value={form.name}
            onChange={set("name")}
            placeholder={t("callbacks.namePlaceholder")}
          />
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">{t("callbacks.transport")}</label>
          <Select
            options={TYPES.map((type) => ({ value: type, label: t(`callbacks.type.${type}`) }))}
            value={form.type}
            onChange={(type) => setForm((current) => ({ ...current, type }))}
          />
        </div>
      </div>

      <div className="sf-field">
        <label className="sf-label">{t("callbacks.url")}</label>
        <input
          className="sf-input"
          value={form.url}
          onChange={set("url")}
          placeholder={URL_HINTS[form.type]}
        />
        <p className="sf-muted" style={{ fontSize: "0.8rem" }}>{t("callbacks.urlHelp")}</p>
      </div>

      {carriesHeaders ? (
        <>
          <div className="sf-field">
            <label className="sf-label">{t("callbacks.headers")}</label>
            <textarea
              className="sf-input"
              rows={2}
              value={form.headersText}
              onChange={set("headersText")}
              placeholder={"X-Device-Key: abc123"}
            />
            <p className="sf-muted" style={{ fontSize: "0.8rem" }}>{t("callbacks.headersHelp")}</p>
          </div>
          <div className="sf-field">
            <label className="sf-label">{t("callbacks.token")}</label>
            <input
              className="sf-input"
              type="password"
              autoComplete="off"
              value={form.token}
              onChange={set("token")}
              placeholder={callback?.hasToken ? t("callbacks.secretSet") : ""}
            />
            <p className="sf-muted" style={{ fontSize: "0.8rem" }}>
              {callback?.hasToken ? t("callbacks.secretKeep") : t("callbacks.tokenHelp")}
            </p>
          </div>
        </>
      ) : null}

      {isMqtt ? (
        <div className="sf-row" style={{ gap: "0.6rem", flexWrap: "wrap" }}>
          <div className="sf-field" style={{ flex: 2, minWidth: 200 }}>
            <label className="sf-label">{t("callbacks.topic")}</label>
            <input className="sf-input" value={form.topic} onChange={set("topic")} placeholder="strongfish/bar-speed" />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 120 }}>
            <label className="sf-label">{t("callbacks.clientId")}</label>
            <input className="sf-input" value={form.clientId} onChange={set("clientId")} placeholder="strongfish" />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 100 }}>
            <label className="sf-label">{t("callbacks.qos")}</label>
            <Select
              options={["0", "1", "2"].map((value) => ({ value, label: value }))}
              value={form.qos}
              onChange={(qos) => setForm((current) => ({ ...current, qos }))}
            />
          </div>
        </div>
      ) : null}

      <div className="sf-row" style={{ gap: "0.6rem", flexWrap: "wrap" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">
            {isHttp ? t("callbacks.basicUser") : t("callbacks.username")}
          </label>
          <input className="sf-input" autoComplete="off" value={form.username} onChange={set("username")} />
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 160 }}>
          <label className="sf-label">
            {isHttp ? t("callbacks.basicPassword") : t("callbacks.password")}
          </label>
          <input
            className="sf-input"
            type="password"
            autoComplete="off"
            value={form.password}
            onChange={set("password")}
            placeholder={callback?.hasPassword ? t("callbacks.secretSet") : ""}
          />
        </div>
      </div>
      <p className="sf-muted" style={{ fontSize: "0.8rem", marginTop: 0 }}>
        {isHttp ? t("callbacks.basicAuthHelp") : t("callbacks.mqttAuthHelp")}
      </p>

      <label className="sf-row" style={{ gap: "0.5rem", alignItems: "center", marginTop: "0.4rem" }}>
        <Switch
          checked={form.enabled}
          onChange={(value) => setForm((current) => ({ ...current, enabled: value }))}
        />
        <span>{t("callbacks.enabled")}</span>
      </label>

      <div className="sf-row" style={{ gap: "0.5rem", marginTop: "0.6rem" }}>
        <button className="sf-button" type="submit" disabled={busy}>
          {t("common.save")}
        </button>
        <button className="sf-button-ghost" type="button" onClick={onCancel}>
          {t("common.cancel")}
        </button>
      </div>
    </form>
  );
}

/** Headers as "Name: value" lines for the textarea. */
function headersToText(headers) {
  if (!Array.isArray(headers)) return "";
  return headers.map((header) => `${header.name}: ${header.value}`).join("\n");
}

/** Back to the array the API takes, dropping blank lines and ones with no name. */
function textToHeaders(text) {
  return (text || "")
    .split("\n")
    .map((line) => {
      const at = line.indexOf(":");
      if (at < 0) return { name: line.trim(), value: "" };
      return { name: line.slice(0, at).trim(), value: line.slice(at + 1).trim() };
    })
    .filter((header) => header.name !== "");
}
