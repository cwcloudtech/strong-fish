import { useCallback, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import ReactMarkdown from "react-markdown";
import { FiCheck, FiEdit2, FiLink, FiPlus, FiSend, FiTrash2, FiX } from "react-icons/fi";

import ExportMenu from "../../components/programs/ExportMenu";
import Select from "../../components/common/Select";
import useToastError from "../../utils/useToast";
import { connectionLabel } from "../../components/ai/AskButton";
import ShareButtons from "../../components/common/ShareButtons";
import downloadBlob from "../../utils/downloadBlob";
import toastOptions from "../../utils/toastOptions";
import { ai as aiApi } from "../../api/services";
import { EmptyState, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The AI coach: a thread of questions, and the threads already had.
 *
 * Every exchange is kept server-side, which is why the list on the left is the
 * member's history rather than this tab's. A chat that forgets itself on
 * reload is one nobody uses twice, and an answer that cost tokens to produce
 * is worth more than a browser tab.
 */
export default function Coach() {
  const { t } = useI18n();
  const toastError = useToastError();
  const [conversations, setConversations] = useState(null);
  const [current, setCurrent] = useState(null);
  const [question, setQuestion] = useState("");
  const [asking, setAsking] = useState(false);
  const [error, setError] = useState(null);
  // The subscriptions this member may spend, and which one the next question
  // goes through. A picker only matters to somebody with more than one - a
  // coach's key beside their own - so with one it is simply not drawn.
  const [connections, setConnections] = useState([]);
  const [connectionId, setConnectionId] = useState("");
  // Which threads the member has ticked, for deleting several at once.
  const [picked, setPicked] = useState([]);
  const [exporting, setExporting] = useState(false);
  const [renaming, setRenaming] = useState(null);
  const bottom = useRef(null);

  const loadList = useCallback(async () => {
    try {
      setConversations(await aiApi.conversations());
    } catch {
      setConversations([]);
    }
  }, []);

  useEffect(() => {
    loadList();
    aiApi
      .connections()
      .then((list) => {
        setConnections(list);
        setConnectionId(list[0]?.id || "");
      })
      .catch(() => setConnections([]));
  }, [loadList]);

  // The newest turn, every time one arrives: a thread that answers below the
  // fold looks like it did not answer.
  useEffect(() => {
    bottom.current?.scrollIntoView({ behavior: "smooth" });
  }, [current, asking]);

  const open = async (conversationId) => {
    setError(null);
    try {
      setCurrent(await aiApi.conversation(conversationId));
    } catch (err) {
      toastError(err);
    }
  };

  const ask = async (event) => {
    event.preventDefault();
    const text = question.trim();
    if (!text || asking) return;

    setAsking(true);
    setError(null);
    // Shown immediately, with the answer blank: the model takes seconds, and
    // a composer that empties with nothing in its place reads as a failure.
    const pending = { id: "pending", question: text, answer: "" };
    setCurrent((thread) => ({
      ...(thread || { title: text, exchanges: [] }),
      exchanges: [...(thread?.exchanges || []), pending],
    }));
    setQuestion("");

    try {
      const answer = await aiApi.ask(text, current?.id, connectionId);
      setCurrent(await aiApi.conversation(answer.conversationId));
      loadList();
    } catch (err) {
      // Only the "nothing connected yet" case stays on the page: it is a
      // state rather than an incident, and it carries the link that fixes
      // it. Everything else is a toast, so it is not said twice.
      setError(err?.response?.data?.i18n_code === "errors.aiNotConfigured" ? err : null);
      toastError(err);
      // The question goes back in the box rather than being lost with the
      // turn that never happened.
      setQuestion(text);
      setCurrent((thread) =>
        thread ? { ...thread, exchanges: thread.exchanges.filter((one) => one.id !== "pending") } : null
      );
    } finally {
      setAsking(false);
    }
  };

  const rename = async (title) => {
    setRenaming(null);
    if (!title.trim() || !current) return;
    try {
      setCurrent(await aiApi.rename(current.id, title.trim()));
      loadList();
    } catch (err) {
      toastError(err);
    }
  };

  const publish = async () => {
    try {
      setCurrent(await aiApi.publish(current.id));
      loadList();
    } catch (err) {
      toastError(err);
    }
  };

  const unpublish = async () => {
    try {
      await aiApi.unpublish(current.id);
      setCurrent({ ...current, shared: false, shareToken: "", shareUrl: "" });
      loadList();
    } catch (err) {
      toastError(err);
    }
  };

  const exportAs = async (format) => {
    setExporting(true);
    try {
      const blob = await aiApi.exportConversation(current.id, format);
      downloadBlob(blob, `${(current.title || "conversation").replace(/[^\w-]+/g, "-")}.${format}`);
    } catch (err) {
      toastError(err);
    } finally {
      setExporting(false);
    }
  };

  const remove = async (conversationId) => {
    try {
      await aiApi.remove(conversationId);
      if (current?.id === conversationId) setCurrent(null);
      setPicked((ids) => ids.filter((id) => id !== conversationId));
      loadList();
    } catch (err) {
      toastError(err);
    }
  };

  const removePicked = async () => {
    try {
      const { deleted } = await aiApi.removeMany(picked);
      if (picked.includes(current?.id)) setCurrent(null);
      setPicked([]);
      loadList();
      toast.success(t("ai.deletedCount", { count: deleted }), toastOptions);
    } catch (err) {
      toastError(err);
    }
  };

  const clearAll = async () => {
    // eslint-disable-next-line no-restricted-globals
    if (!window.confirm(t("ai.clearAllConfirm"))) return;
    try {
      const { deleted } = await aiApi.clearAll();
      setCurrent(null);
      setPicked([]);
      loadList();
      toast.success(t("ai.deletedCount", { count: deleted }), toastOptions);
    } catch (err) {
      toastError(err);
    }
  };

  if (!conversations) return <Spinner />;

  const pick = (conversationId) =>
    setPicked((ids) =>
      ids.includes(conversationId) ? ids.filter((id) => id !== conversationId) : [...ids, conversationId]
    );

  return (
    <div className="sf-page">
      <div className="sf-page-header">
        <div>
          <h1>{t("ai.coachTitle")}</h1>
          <p className="sf-subtitle">{t("ai.coachSubtitle")}</p>
        </div>
        <div className="sf-row" style={{ gap: "0.5rem", alignItems: "center", flexWrap: "wrap" }}>
          {/* Which subscription the next question is spent on. Drawn only
              when there is a choice to make. */}
          {connections.length > 1 ? (
            <div style={{ minWidth: 200 }}>
              <Select
                options={connections.map((connection) => ({
                  value: connection.id,
                  label: connectionLabel(connection, t),
                }))}
                value={connectionId}
                onChange={setConnectionId}
                placeholder={t("ai.connection")}
              />
            </div>
          ) : null}
          <button type="button" className="sf-button" onClick={() => setCurrent(null)}>
            <FiPlus /> {t("ai.newConversation")}
          </button>
        </div>
      </div>

      {/* Nothing connected: the page is useless until it is, and the member
          is one click from the screen that fixes it. */}
      {error ? (
        <div className="sf-notice sf-notice-warning">
          <div className="sf-row-between">
            <span>{t("errors.aiNotConfigured")}</span>
            <Link className="sf-button sf-button-sm" to="/dashboard/settings">
              {t("ai.connectOne")}
            </Link>
          </div>
        </div>
      ) : null}

      <div className="sf-messages">
        <div>
          <div className="sf-messages-list">
            {conversations.length === 0 ? (
              <p className="sf-muted" style={{ padding: "0.8rem" }}>{t("ai.noConversations")}</p>
            ) : (
              conversations.map((conversation) => (
                <div
                  key={conversation.id}
                  className={`sf-conversation ${current?.id === conversation.id ? "active" : ""}`}
                >
                  <input
                    type="checkbox"
                    checked={picked.includes(conversation.id)}
                    onChange={() => pick(conversation.id)}
                    aria-label={t("ai.pickConversation")}
                  />
                  <button
                    type="button"
                    className="sf-conversation-body"
                    style={{ border: 0, background: "none", cursor: "pointer", textAlign: "left" }}
                    onClick={() => open(conversation.id)}
                  >
                    <span className="sf-conversation-name">{conversation.title}</span>
                    <span className="sf-muted">
                      {conversation.count === 1
                        ? t("ai.exchangeOne")
                        : t("ai.exchangeCount", { count: conversation.count })}
                      {conversation.shared ? ` · ${t("ai.sharedBadge")}` : ""}
                    </span>
                  </button>
                  <button
                    type="button"
                    className="sf-button-ghost sf-button-sm"
                    onClick={() => remove(conversation.id)}
                    aria-label={t("common.delete")}
                    title={t("common.delete")}
                  >
                    <FiTrash2 />
                  </button>
                </div>
              ))
            )}
          </div>

          {conversations.length > 0 ? (
            <div className="sf-row" style={{ gap: "0.4rem", marginTop: "0.5rem", flexWrap: "wrap" }}>
              <button
                type="button"
                className="sf-button-ghost sf-button-sm"
                onClick={removePicked}
                disabled={picked.length === 0}
              >
                <FiTrash2 /> {t("ai.deletePicked", { count: picked.length })}
              </button>
              <button type="button" className="sf-button-ghost sf-button-sm" onClick={clearAll}>
                {t("ai.clearAll")}
              </button>
            </div>
          ) : null}
        </div>

        <div className="sf-messages-thread">
          <div className="sf-thread-header">
            {renaming !== null ? (
              <>
                <input
                  className="sf-input"
                  value={renaming}
                  autoFocus
                  onChange={(event) => setRenaming(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") rename(renaming);
                    if (event.key === "Escape") setRenaming(null);
                  }}
                />
                <button
                  type="button"
                  className="sf-button-ghost sf-button-sm"
                  onClick={() => rename(renaming)}
                  aria-label={t("common.save")}
                >
                  <FiCheck />
                </button>
                <button
                  type="button"
                  className="sf-button-ghost sf-button-sm"
                  onClick={() => setRenaming(null)}
                  aria-label={t("common.cancel")}
                >
                  <FiX />
                </button>
              </>
            ) : (
              <>
                <strong style={{ flex: 1, minWidth: 0 }}>
                  {current?.title || t("ai.newConversation")}
                </strong>
                {current ? (
                  <>
                    <button
                      type="button"
                      className="sf-button-ghost sf-button-sm"
                      onClick={() => setRenaming(current.title || "")}
                      aria-label={t("ai.rename")}
                      title={t("ai.rename")}
                    >
                      <FiEdit2 />
                    </button>
                    <button
                      type="button"
                      className={`sf-button-ghost sf-button-sm ${current.shared ? "is-active" : ""}`}
                      onClick={current.shared ? unpublish : publish}
                      aria-label={current.shared ? t("ai.unshareLink") : t("ai.shareLink")}
                      title={current.shared ? t("ai.unshareLink") : t("ai.shareLink")}
                    >
                      <FiLink />
                    </button>
                    <ExportMenu
                      onExport={exportAs}
                      busy={exporting}
                      label={t("ai.exportConversation")}
                      size="sf-button-sm"
                      formats={["pdf", "png"]}
                    />
                  </>
                ) : null}
              </>
            )}
          </div>

          {/* The link, once there is one: anybody holding it can read the
              thread, which is exactly what sharing one means. */}
          {current?.shareUrl ? (
            <div className="sf-notice" style={{ margin: "0.6rem" }}>
              <p className="sf-muted" style={{ marginTop: 0 }}>{t("ai.shareLinkHelp")}</p>
              <ShareButtons url={current.shareUrl} text={current.title} label={t("ai.shareLink")} />
            </div>
          ) : null}

          <div className="sf-thread-messages">
            {!current || current.exchanges?.length === 0 ? (
              <EmptyState title={t("ai.emptyTitle")} message={t("ai.emptyBody")} />
            ) : (
              (current.exchanges || []).map((exchange) => (
                <div key={exchange.id}>
                  <div className="sf-bubble-row mine">
                    <div className="sf-bubble">{exchange.question}</div>
                  </div>
                  <div className="sf-bubble-row">
                    <div className="sf-bubble sf-bubble-markdown">
                      {exchange.answer ? (
                        <ReactMarkdown>{exchange.answer}</ReactMarkdown>
                      ) : (
                        <span className="sf-muted">{t("ai.thinking")}</span>
                      )}
                    </div>
                  </div>
                </div>
              ))
            )}
            <div ref={bottom} />
          </div>

          <form className="sf-thread-composer" onSubmit={ask}>
            <textarea
              className="sf-textarea"
              rows={2}
              style={{ flex: 1 }}
              value={question}
              placeholder={t("ai.placeholder")}
              onChange={(event) => setQuestion(event.target.value)}
              onKeyDown={(event) => {
                // Enter sends, shift+enter makes a line: the convention every
                // chat has trained people on.
                if (event.key === "Enter" && !event.shiftKey) ask(event);
              }}
            />
            <button className="sf-button" type="submit" disabled={asking || !question.trim()}>
              <FiSend /> {t("ai.send")}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
