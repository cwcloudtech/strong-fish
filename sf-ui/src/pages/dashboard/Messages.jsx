import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { toast } from "react-toastify";
import { FiFlag, FiSearch, FiSend, FiSlash, FiUser } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import Avatar from "../../components/common/Avatar";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import Tooltip from "../../components/common/Tooltip";
import { blocks as blocksApi, messages as messagesApi, social } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Private messages: the list of conversations on the left, the open thread on
 * the right.
 *
 * Which thread is open lives in the URL (`?with=`), so a link from a profile
 * opens straight into it and the back button does what it looks like it does.
 */
export default function Messages() {
  const { t } = useI18n();
  const [params, setParams] = useSearchParams();
  const openWith = params.get("with") || "";

  const [conversations, setConversations] = useState(null);
  const [error, setError] = useState(null);

  const loadConversations = useCallback(async () => {
    try {
      setConversations(await messagesApi.conversations());
    } catch (err) {
      setError(err);
    }
  }, []);

  useEffect(() => {
    loadConversations();
  }, [loadConversations]);

  return (
    <div className="sf-page" style={{ maxWidth: 1000 }}>
      <div className="sf-page-header">
        <div>
          <h1 className="sf-title">{t("messages.title")}</h1>
          <p className="sf-subtitle">{t("messages.subtitle")}</p>
        </div>
        <div className="sf-row" style={{ gap: "0.4rem" }}>
          {/* Starting a conversation means finding somebody first, and there is
              nowhere else on this screen to do it - every thread here already
              exists. */}
          <Link className="sf-button" to="/dashboard/search">
            <FiSearch /> {t("messages.findSomeone")}
          </Link>
          <Link className="sf-button sf-button-secondary" to="/dashboard/blocks">
            <FiSlash /> {t("blocks.title")}
          </Link>
        </div>
      </div>

      <ErrorMessage error={error} />

      <div className="sf-messages">
        <div className="sf-messages-list">
          {conversations === null ? (
            <Spinner />
          ) : conversations.length === 0 ? (
            <EmptyState title={t("messages.emptyTitle")} message={t("messages.emptyBody")}>
              <Link className="sf-button sf-button-sm" to="/dashboard/search">
                <FiSearch /> {t("messages.findSomeone")}
              </Link>
            </EmptyState>
          ) : (
            conversations.map((conversation) => (
              <button
                type="button"
                key={conversation.id}
                className={`sf-conversation ${conversation.other.id === openWith ? "active" : ""}`}
                onClick={() => setParams({ with: conversation.other.id })}
              >
                <Avatar user={conversation.other} size="sf-avatar-sm" />
                <span className="sf-conversation-body">
                  <strong>
                    {conversation.other.name} {conversation.other.surname}
                  </strong>
                  <span className="sf-muted">{conversation.lastMessage}</span>
                </span>
                {conversation.unread ? <span className="sf-nav-count">{conversation.unread}</span> : null}
              </button>
            ))
          )}
        </div>

        <div className="sf-messages-thread">
          {openWith ? (
            <Thread userId={openWith} onSent={loadConversations} />
          ) : (
            <EmptyState title={t("messages.pickTitle")} message={t("messages.pickBody")} />
          )}
        </div>
      </div>
    </div>
  );
}

/** One open conversation. */
function Thread({ userId, onSent }) {
  const { t } = useI18n();
  const [thread, setThread] = useState(null);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [reporting, setReporting] = useState(null);
  const [blocking, setBlocking] = useState(false);
  const bottom = useRef(null);

  const load = useCallback(async () => {
    setError(null);
    try {
      setThread(await messagesApi.thread(userId));
    } catch (err) {
      setThread(null);
      setError(err);
    }
  }, [userId]);

  useEffect(() => {
    load();
  }, [load]);

  // A conversation is read from the bottom: the newest message is the one you
  // opened it for.
  useEffect(() => {
    bottom.current?.scrollIntoView({ block: "end" });
  }, [thread]);

  const send = async (event) => {
    event.preventDefault();
    const content = draft.trim();
    if (!content) return;
    setBusy(true);
    try {
      const message = await messagesApi.send(userId, content);
      setThread((current) => ({ ...current, messages: [...(current?.messages || []), message] }));
      setDraft("");
      onSent();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const block = async () => {
    try {
      await blocksApi.block(userId);
      toast.success(t("blocks.blocked"), toastOptions);
      setBlocking(false);
      onSent();
      await load();
    } catch (err) {
      setError(err);
    }
  };

  if (error && !thread) return <ErrorMessage error={error} />;
  if (!thread) return <Spinner />;

  return (
    <>
      <div className="sf-thread-header">
        <Avatar user={thread.other} size="sf-avatar-sm" />
        <strong style={{ flex: 1, minWidth: 0 }}>
          {thread.other.name} {thread.other.surname}
        </strong>
        {thread.other.handle ? (
          <Tooltip label={t("search.openProfile")}>
            <Link className="sf-icon-button sf-icon-button-plain" to={`/profile/${thread.other.handle}`}>
              <FiUser />
            </Link>
          </Tooltip>
        ) : null}
        <Tooltip label={t("blocks.block")}>
          <button
            type="button"
            className="sf-icon-button sf-icon-button-plain"
            onClick={() => setBlocking(true)}
            aria-label={t("blocks.block")}
          >
            <FiSlash />
          </button>
        </Tooltip>
      </div>

      <div className="sf-thread-messages">
        {thread.messages.length === 0 ? (
          <p className="sf-muted" style={{ textAlign: "center" }}>
            {t("messages.threadEmpty")}
          </p>
        ) : (
          thread.messages.map((message) => (
            <div key={message.id} className={`sf-bubble-row ${message.mine ? "mine" : ""}`}>
              <div className="sf-bubble">
                <p style={{ whiteSpace: "pre-wrap", margin: 0 }}>{message.content}</p>
                <span className="sf-bubble-time">
                  {new Date(message.createdAt).toLocaleString()}
                </span>
              </div>
              {/* Reporting your own message would be reporting yourself. */}
              {message.mine ? null : (
                <Tooltip label={t("messages.report")}>
                  <button
                    type="button"
                    className="sf-icon-button sf-icon-button-plain"
                    onClick={() => setReporting(message)}
                    aria-label={t("messages.report")}
                  >
                    <FiFlag />
                  </button>
                </Tooltip>
              )}
            </div>
          ))
        )}
        <div ref={bottom} />
      </div>

      <ErrorMessage error={error} />

      <form className="sf-thread-composer" onSubmit={send}>
        <textarea
          className="sf-textarea"
          rows={2}
          value={draft}
          placeholder={t("messages.placeholder")}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            // Enter sends, shift+enter breaks the line - what every messenger
            // trains people to expect.
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              send(event);
            }
          }}
        />
        <button className="sf-button" type="submit" disabled={busy || !draft.trim()}>
          <FiSend /> {t("messages.send")}
        </button>
      </form>

      {reporting ? (
        <ReportMessageModal
          message={reporting}
          onClose={() => setReporting(null)}
          onReported={() => {
            setReporting(null);
            toast.success(t("messages.reported"), toastOptions);
          }}
        />
      ) : null}

      {blocking ? (
        <ConfirmModal
          title={t("blocks.blockTitle")}
          message={t("blocks.blockBody", { name: `${thread.other.name} ${thread.other.surname}`.trim() })}
          confirmLabel={t("blocks.block")}
          onConfirm={block}
          onClose={() => setBlocking(false)}
        />
      ) : null}
    </>
  );
}

/**
 * Reports one message to the moderators. A moderator cannot open a private
 * thread, so the API stores the message's text with the report - which is why
 * this needs no extra context from the reporter beyond a reason.
 */
function ReportMessageModal({ message, onClose, onReported }) {
  const { t } = useI18n();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    try {
      await social.report({ targetType: "message", targetId: message.id, reason });
      onReported();
    } catch (err) {
      toast.error(err?.response?.data?.message || t("errors.generic"), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      title={t("messages.reportTitle")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button type="submit" form="sf-report-message" className="sf-button" disabled={busy || !reason.trim()}>
            {t("messages.report")}
          </button>
        </>
      }
    >
      <p className="sf-muted" style={{ marginTop: 0 }}>
        {t("messages.reportHelp")}
      </p>
      <form id="sf-report-message" onSubmit={submit}>
        <textarea
          className="sf-textarea"
          rows={3}
          autoFocus
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          placeholder={t("messages.reportPlaceholder")}
        />
      </form>
    </Modal>
  );
}
