import { useCallback, useEffect, useState } from "react";
import { toast } from "react-toastify";
import { FiArrowDown, FiArrowUp, FiEdit2, FiPlus, FiTrash2, FiUsers } from "react-icons/fi";

import AutocompleteInput from "../common/AutocompleteInput";
import MultiSelect from "../common/MultiSelect";
import Select from "../common/Select";
import toastOptions from "../../utils/toastOptions";
import { ai as aiApi, search as searchApi } from "../../api/services";
import useToastError from "../../utils/useToast";
import { ErrorMessage } from "../common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The member's connections to a language model, and who else may spend them.
 *
 * Deliberately the storage settings over again: an account holds a credential
 * and decides who else may use it. A coach paying for one key and letting
 * their athletes ask it questions is the same arrangement as a club sharing a
 * bucket, and a second shape for it would be a second thing to learn.
 */
export default function AiConnections() {
  const { t } = useI18n();
  const { user } = useAuth();
  const toastError = useToastError();
  const [connections, setConnections] = useState(null);
  const [providers, setProviders] = useState([]);
  const [busy, setBusy] = useState(false);
  // Which connection's form is open: an id, "new" while adding one, or null.
  // One at a time - two open key forms is two ways to lose track of which
  // account you are editing.
  const [editing, setEditing] = useState(null);
  const [sharing, setSharing] = useState(null);

  const load = useCallback(async () => {
    try {
      setConnections(await aiApi.connections());
    } catch {
      setConnections([]);
    }
  }, []);

  useEffect(() => {
    load();
    aiApi.providers().then(setProviders).catch(() => setProviders([]));
  }, [load]);

  if (!connections) return null;

  // The member's own, in order. The ones shared with them follow in the list
  // and have no arrows: the owner decides where those sit.
  const mine = connections.filter((connection) => connection.ownerId === user?.id);

  // Moving one is a swap with its neighbour, sent as the whole order: the
  // API takes a list, so there is one way for the order to be wrong rather
  // than two. Only the member's own are moved - a connection somebody shared
  // with them sits in their list but is not theirs to order.
  const move = (connectionId, by) => {
    const ids = mine.map((connection) => connection.id);
    const from = ids.indexOf(connectionId);
    const to = from + by;
    if (from < 0 || to < 0 || to >= ids.length) return;
    [ids[from], ids[to]] = [ids[to], ids[from]];
    act(() => aiApi.reorderConnections(ids));
  };

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

  return (
    <div className="sf-card">
      <h2>{t("ai.title")}</h2>
      <p className="sf-subtitle">{t("ai.subtitle")}</p>

      {connections.length === 0 ? (
        <p className="sf-muted">{t("ai.noConnections")}</p>
      ) : (
        <>
          {mine.length > 1 ? (
            <p className="sf-muted" style={{ fontSize: "0.85rem" }}>{t("ai.orderHelp")}</p>
          ) : null}
        <ul className="sf-list">
          {connections.map((connection) => {
            // Shared with me, not mine: it can be spent, never edited or
            // re-shared, so it is listed with its owner's name and no
            // controls at all.
            const own = connection.ownerId === user?.id;
            // Its place in the member's own order, for the arrows; -1 for a
            // connection somebody shared with them.
            const index = mine.findIndex((candidate) => candidate.id === connection.id);
            // The vendor's own name, from the provider catalogue: it is
            // branding rather than a translated string, and a copy of it in
            // three dictionaries would be a second list to keep in step.
            const label = providerLabel(providers, connection.provider);
            return (
              <li className="sf-list-item" key={connection.id} style={{ flexWrap: "wrap" }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <strong>{connection.name || label}</strong>
                  <div className="sf-muted" style={{ fontSize: "0.85rem" }}>
                    {[
                      label,
                      connection.model,
                      own && index === 0 ? t("ai.first") : null,
                      own ? null : t("ai.sharedBy", { name: connection.ownerName }),
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </div>
                </div>

                {own ? (
                  <div className="sf-row" style={{ gap: "0.25rem" }}>
                    <button
                      type="button"
                      className="sf-button-ghost sf-button-sm"
                      onClick={() => move(connection.id, -1)}
                      disabled={busy || index === 0}
                      aria-label={t("ai.moveUp")}
                      title={t("ai.moveUp")}
                    >
                      <FiArrowUp />
                    </button>
                    <button
                      type="button"
                      className="sf-button-ghost sf-button-sm"
                      onClick={() => move(connection.id, 1)}
                      disabled={busy || index === mine.length - 1}
                      aria-label={t("ai.moveDown")}
                      title={t("ai.moveDown")}
                    >
                      <FiArrowDown />
                    </button>
                    <button
                      type="button"
                      className="sf-button-ghost sf-button-sm"
                      onClick={() => setSharing(sharing === connection.id ? null : connection.id)}
                      aria-label={t("ai.shares")}
                      title={t("ai.shares")}
                    >
                      <FiUsers />
                    </button>
                    <button
                      type="button"
                      className="sf-button-ghost sf-button-sm"
                      onClick={() => setEditing(editing === connection.id ? null : connection.id)}
                      aria-label={t("common.edit")}
                      title={t("common.edit")}
                    >
                      <FiEdit2 />
                    </button>
                    <button
                      type="button"
                      className="sf-button-ghost sf-button-sm"
                      onClick={() => act(() => aiApi.clearConnection(connection.id), t("ai.cleared"))}
                      disabled={busy}
                      aria-label={t("ai.clear")}
                      title={t("ai.clear")}
                    >
                      <FiTrash2 />
                    </button>
                  </div>
                ) : (
                  <span className="sf-badge sf-badge-muted">{t("ai.role.user")}</span>
                )}

                {editing === connection.id ? (
                  <div style={{ flexBasis: "100%" }}>
                    <AiConnectionForm
                      connection={connection}
                      providers={providers}
                      busy={busy}
                      onCancel={() => setEditing(null)}
                      onSubmit={(payload) =>
                        act(() => aiApi.setConnection(connection.id, payload), t("ai.saved"))
                      }
                    />
                  </div>
                ) : null}

                {sharing === connection.id ? (
                  <div style={{ flexBasis: "100%" }}>
                    <AiShares connectionId={connection.id} />
                  </div>
                ) : null}
              </li>
            );
          })}
        </ul>
        </>
      )}

      {editing === "new" ? (
        <AiConnectionForm
          connection={null}
          providers={providers}
          busy={busy}
          onCancel={() => setEditing(null)}
          onSubmit={(payload) => act(() => aiApi.addConnection(payload), t("ai.saved"))}
        />
      ) : (
        <button type="button" className="sf-button" onClick={() => setEditing("new")}>
          <FiPlus /> {t("ai.addConnection")}
        </button>
      )}
    </div>
  );
}

/**
 * The connection form, for a new one or one being edited.
 *
 * The address is a field rather than a constant because that is the point:
 * picking Anthropic and then pointing it at a LiteLLM on the next machine is
 * the arrangement this was built for. Picking a provider fills the address and
 * the model in, so somebody who only has a key is one paste from done.
 */
function AiConnectionForm({ connection, providers, busy, onCancel, onSubmit }) {
  const { t } = useI18n();
  // The random half of the suggested name, minted once for this form: it must
  // not change under the member while they look at it, and two connections to
  // the same model have to be tellable apart.
  const [suffix] = useState(randomSuffix);
  const [form, setForm] = useState({
    name: connection?.name || "",
    provider: connection?.provider || providers[0]?.name || "",
    baseUrl: connection?.baseUrl || "",
    model: connection?.model || "",
    token: "",
  });

  // Everything the form shows about a provider - its address, its models, what
  // its key looks like - comes from this row. The placeholders change with the
  // picker because they are that provider's own values, not a fixed example of
  // somebody else's.
  const defaults = providers.find((provider) => provider.name === form.provider);

  const set = (field) => (event) =>
    setForm((current) => ({ ...current, [field]: event.target.value }));

  // Switching provider rewrites the address and the model, unless the member
  // has typed their own: changing vendor and keeping the old vendor's URL is
  // never what was meant, but neither is losing a hand-typed gateway.
  const setProvider = (provider) =>
    setForm((current) => {
      const previous = providers.find((entry) => entry.name === current.provider);
      const next = providers.find((entry) => entry.name === provider);
      const kept = (value, was, now) => (value && value !== was ? value : now || "");
      return {
        ...current,
        provider,
        baseUrl: kept(current.baseUrl, previous?.defaultUrl, next?.defaultUrl),
        model: kept(current.model, previous?.defaultModel, next?.defaultModel),
      };
    });

  // What the name field offers, and what a blank one becomes: the model it
  // points at, plus the suffix. The API fills a blank name exactly this way,
  // so what the placeholder promised is what gets saved.
  const model = form.model.trim() || defaults?.defaultModel || "";
  const suggestedName = model ? `${model}-${suffix}` : "";

  const submit = (event) => {
    event.preventDefault();
    onSubmit({
      name: form.name.trim() || suggestedName,
      provider: form.provider,
      baseUrl: form.baseUrl.trim() || defaults?.defaultUrl || "",
      model,
      token: form.token.trim(),
    });
  };

  return (
    <form className="sf-card" style={{ marginTop: "0.8rem" }} onSubmit={submit}>
      <div className="sf-row" style={{ gap: "0.6rem", flexWrap: "wrap" }}>
        <div className="sf-field" style={{ flex: 1, minWidth: 180 }}>
          <label className="sf-label">{t("ai.name")}</label>
          <input
            className="sf-input"
            value={form.name}
            onChange={set("name")}
            placeholder={suggestedName || t("ai.namePlaceholder", { provider: defaults?.label || form.provider })}
          />
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 180 }}>
          <label className="sf-label">{t("ai.providerLabel")}</label>
          <Select
            options={providers.map((provider) => ({ value: provider.name, label: provider.label }))}
            value={form.provider}
            onChange={setProvider}
          />
        </div>
      </div>

      <div className="sf-row" style={{ gap: "0.6rem", flexWrap: "wrap" }}>
        <div className="sf-field" style={{ flex: 2, minWidth: 220 }}>
          <label className="sf-label">{t("ai.baseUrl")}</label>
          <input
            className="sf-input"
            value={form.baseUrl}
            onChange={set("baseUrl")}
            placeholder={defaults?.defaultUrl || "https://"}
          />
          <p className="sf-muted" style={{ fontSize: "0.8rem" }}>{t("ai.baseUrlHelp")}</p>
        </div>
        <div className="sf-field" style={{ flex: 1, minWidth: 180 }}>
          <label className="sf-label">{t("ai.model")}</label>
          {/* Suggestions, not a list of permitted values: a provider ships a
              model the day before the catalogue hears about it, and the API
              takes whatever is typed. */}
          <AutocompleteInput
            value={form.model}
            onChange={(model) => setForm((current) => ({ ...current, model }))}
            options={defaults?.models || []}
            placeholder={defaults?.defaultModel || ""}
          />
          <p className="sf-muted" style={{ fontSize: "0.8rem" }}>{t("ai.modelHelp")}</p>
        </div>
      </div>

      <div className="sf-field">
        <label className="sf-label">{t("ai.token")}</label>
        <input
          className="sf-input"
          type="password"
          autoComplete="off"
          value={form.token}
          onChange={set("token")}
          placeholder={connection?.hasToken ? t("ai.tokenSet") : defaults?.keyHint || ""}
        />
        <p className="sf-muted" style={{ fontSize: "0.8rem" }}>
          {connection?.hasToken ? t("ai.tokenKeep") : t("ai.tokenHelp")}
        </p>
      </div>

      <div className="sf-row" style={{ gap: "0.5rem" }}>
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

/** Who else may put questions through this connection. */
function AiShares({ connectionId }) {
  const { t } = useI18n();
  const toastError = useToastError();
  const [grants, setGrants] = useState(null);
  const [userIds, setUserIds] = useState([]);
  const [members, setMembers] = useState([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    aiApi.shares(connectionId).then(setGrants).catch(setError);
    searchApi
      .members({ size: 100 })
      .then((page) => setMembers(page?.results || []))
      .catch(() => setMembers([]));
  }, [connectionId]);

  const share = async () => {
    setBusy(true);
    setError(null);
    try {
      let latest = grants;
      // One request per person: granting is rare, and a partial failure then
      // names who it failed for.
      for (const userId of userIds) {
        latest = await aiApi.share(connectionId, userId);
      }
      setGrants(latest);
      setUserIds([]);
      toast.success(t("ai.shared"), toastOptions);
    } catch (err) {
      toastError(err);
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (userId) => {
    setBusy(true);
    try {
      await aiApi.unshare(connectionId, userId);
      setGrants(await aiApi.shares(connectionId));
    } catch (err) {
      toastError(err);
    } finally {
      setBusy(false);
    }
  };

  const shared = (grants || []).filter((grant) => grant.role !== "owner");
  const available = members.filter((member) => !(grants || []).some((grant) => grant.userId === member.id));

  return (
    <div className="sf-card" style={{ marginTop: "0.8rem" }}>
      <h3 style={{ marginTop: 0 }}>{t("ai.shares")}</h3>
      <p className="sf-muted" style={{ marginTop: 0 }}>{t("ai.sharesHelp")}</p>

      {shared.length === 0 ? (
        <p className="sf-muted">{t("ai.noShares")}</p>
      ) : (
        <ul className="sf-list">
          {shared.map((grant) => (
            <li className="sf-list-item" key={grant.userId}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>{grant.name}</strong>
                {grant.handle ? <div className="sf-muted">@{grant.handle}</div> : null}
              </div>
              <span className="sf-badge sf-badge-muted">{t("ai.role.user")}</span>
              <button
                type="button"
                className="sf-button-ghost sf-button-sm"
                onClick={() => revoke(grant.userId)}
                disabled={busy}
                aria-label={t("ai.unshare")}
                title={t("ai.unshare")}
              >
                <FiTrash2 />
              </button>
            </li>
          ))}
        </ul>
      )}

      <div className="sf-row" style={{ gap: "0.5rem", alignItems: "flex-end", flexWrap: "wrap" }}>
        <div className="sf-field" style={{ flex: 2, minWidth: 220, margin: 0 }}>
          <label className="sf-label">{t("ai.shareWith")}</label>
          <MultiSelect
            options={available.map((member) => ({
              value: member.id,
              label: `${member.name} ${member.surname}`.trim() || member.handle,
            }))}
            selected={userIds}
            onChange={setUserIds}
            placeholder={t("ai.pickMembers")}
          />
        </div>
        <button type="button" className="sf-button" onClick={share} disabled={busy || userIds.length === 0}>
          {t("ai.shareAction")}
        </button>
      </div>

      <ErrorMessage error={error} />
    </div>
  );
}

/**
 * A provider's display name, from the catalogue the server serves.
 *
 * Falls back to the stored name for a connection whose provider has since
 * been taken out of the table: it still works, and showing "deepseek" is
 * better than showing nothing.
 */
export function providerLabel(providers, name) {
  return providers.find((provider) => provider.name === name)?.label || name;
}

/**
 * Eight random characters, the tail of a connection's suggested name.
 *
 * Not an id and not a secret - it exists so "claude-sonnet-5-5" twice over
 * reads as two different connections in a menu.
 */
function randomSuffix() {
  const bytes = new Uint8Array(4);
  // crypto is there in every browser this app supports; the fallback is for
  // a test renderer, where a readable name matters more than randomness.
  if (window.crypto?.getRandomValues) {
    window.crypto.getRandomValues(bytes);
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256);
    }
  }
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}
