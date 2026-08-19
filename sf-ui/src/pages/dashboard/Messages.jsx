import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { toast } from "react-toastify";
import { FiFlag, FiImage, FiSearch, FiSend, FiSlash, FiTrash2, FiUser, FiVideo, FiX } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import Avatar from "../../components/common/Avatar";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import Tooltip from "../../components/common/Tooltip";
import { blocks as blocksApi, media as mediaApi, messages as messagesApi, social } from "../../api/services";
import VoiceRecorder from "../../components/messages/VoiceRecorder";
import { readImageAsDataUrl } from "../../utils/image";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";
import LinkifiedText from "../../components/common/LinkifiedText";
import { isFramedMedia } from "../../webcomponents/MediaPlayer";

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
                // Unread threads carry their own state: the count alone is
                // easy to miss down the side of a long list, and what somebody
                // scanning it wants is which rows still need them.
                className={[
                  "sf-conversation",
                  conversation.other.id === openWith ? "active" : "",
                  conversation.unread ? "unread" : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                onClick={() => setParams({ with: conversation.other.id })}
              >
                <Avatar user={conversation.other} size="sf-avatar-sm" />
                <span className="sf-conversation-body">
                  <strong className="sf-conversation-name">
                    {conversation.other.name} {conversation.other.surname}
                  </strong>
                  <span className="sf-muted sf-conversation-preview">{conversation.lastMessage}</span>
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
  const [pictures, setPictures] = useState([]);
  // The recording as made, not yet uploaded: discarding one should never leave
  // a file in the sender's storage.
  const [recording, setRecording] = useState(null);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const fileInput = useRef(null);
  const videoInput = useRef(null);
  // Upload progress, 0..1, or null when nothing is uploading.
  const [uploading, setUploading] = useState(null);
  const [reporting, setReporting] = useState(null);
  const [deleting, setDeleting] = useState(null);
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

  const addPicture = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    try {
      // Read before updating: the updater passed to setPictures is not async,
      // and awaiting inside it is a syntax error rather than a wait.
      const dataUrl = await readImageAsDataUrl(file);
      setPictures((current) => [...current, dataUrl].slice(0, 4));
    } catch (err) {
      setError(err.message === "too-large" ? t("errors.imageTooLarge") : err);
    }
  };

  // Uploading a video appends its URL to the draft rather than attaching it:
  // from there it is an ordinary link, and the same detection that renders a
  // pasted YouTube URL plays it. The feed's composer does exactly this.
  const addVideo = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;

    setUploading(0);
    setError(null);
    try {
      const { url } = await mediaApi.uploadVideo(file, setUploading);
      setDraft((current) => (current.trim() ? `${current.trim()}\n${url}` : url));
    } catch (err) {
      // The API's 405 for "no storage configured" carries its own i18n code,
      // so it reads as "set up your storage first" like every other failure -
      // reported inline here, the way this screen reports the rest.
      setError(err);
    } finally {
      setUploading(null);
    }
  };

  const send = async (event) => {
    event.preventDefault();
    const content = draft.trim();
    if (!content && !pictures.length && !recording) return;

    setBusy(true);
    setError(null);
    try {
      // Uploaded at send time, to the sender's own storage. A 405 here means
      // they have not configured one, and the error says so.
      let audio = "";
      if (recording) {
        const extension = recording.blob.type.includes("mp4") ? "m4a" : "webm";
        const uploaded = await mediaApi.uploadAudio(recording.blob, `voice.${extension}`);
        audio = uploaded.url;
      }

      const message = await messagesApi.send(userId, { content, pictures, audio });
      setThread((current) => ({ ...current, messages: [...(current?.messages || []), message] }));
      setDraft("");
      setPictures([]);
      if (recording?.url) URL.revokeObjectURL(recording.url);
      setRecording(null);
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
        {/* The avatar and the name go through to the profile: it is the first
            thing anybody clicks, and a picture that does nothing reads as
            broken. Only when there is a handle to go to - a profile you cannot
            see has no page to open. */}
        {thread.other.handle ? (
          <Link className="sf-thread-who" to={`/profile/${thread.other.handle}`}>
            <Avatar user={thread.other} size="sf-avatar-sm" />
            <strong>
              {thread.other.name} {thread.other.surname}
            </strong>
          </Link>
        ) : (
          <span className="sf-thread-who">
            <Avatar user={thread.other} size="sf-avatar-sm" />
            <strong>
              {thread.other.name} {thread.other.surname}
            </strong>
          </span>
        )}
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
                {message.content ? (
                  <p style={{ whiteSpace: "pre-wrap", margin: 0 }}>
                    <LinkifiedText text={message.content} />
                  </p>
                ) : null}

                {(message.pictures || []).map((picture, index) => (
                  <img key={index} className="sf-bubble-picture" src={picture} alt="" />
                ))}

                {/* A voice message plays where it sits - the browser's own
                    controls, no player to build. */}
                {message.audio ? (
                  // A voice note in Google Drive is not a file this browser can
                  // fetch: the API stores Drive's /preview URL, which is an
                  // embed page, and an <audio src> pointed at an HTML page
                  // simply never plays. Framed through the media player in that
                  // case, and played natively when the storage serves the file
                  // itself - which is what an S3 bucket does.
                  isFramedMedia(message.audio) ? (
                    <media-player url={message.audio} />
                  ) : (
                    <audio className="sf-bubble-audio" src={message.audio} controls preload="metadata" />
                  )
                ) : null}

                {/* The same detection the feed uses, so a video shared in a
                    thread renders as a player rather than as a bare URL. */}
                {(message.links || []).map((link) => (
                  <media-player key={link} url={link} />
                ))}

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

              {/* Offered on the API's say-so rather than on `mine`: a
                  superadmin may remove anything, and the two clients should
                  not each carry their own copy of that rule. */}
              {message.deletable ? (
                <Tooltip label={t("common.delete")}>
                  <button
                    type="button"
                    className="sf-icon-button sf-icon-button-plain"
                    onClick={() => setDeleting(message)}
                    aria-label={t("common.delete")}
                  >
                    <FiTrash2 />
                  </button>
                </Tooltip>
              ) : null}
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

        {uploading !== null ? (
          <p className="sf-muted" style={{ margin: "0.4rem 0 0" }}>
            {t("feed.uploadingVideo", { percent: uploading })}
          </p>
        ) : null}

        <div className="sf-thread-tools">
          <button
            type="button"
            className="sf-button-ghost"
            onClick={() => fileInput.current?.click()}
            disabled={pictures.length >= 4}
            aria-label={t("feed.addPicture")}
          >
            <FiImage />
          </button>
          <input ref={fileInput} type="file" accept="image/*" hidden onChange={addPicture} />

          <button
            type="button"
            className="sf-button-ghost"
            onClick={() => videoInput.current?.click()}
            disabled={uploading !== null}
            aria-label={t("feed.addVideo")}
          >
            <FiVideo />
          </button>
          <input ref={videoInput} type="file" accept="video/*" hidden onChange={addVideo} />

          <VoiceRecorder recording={recording} onRecordingChange={setRecording} disabled={busy} />

          <button
            className="sf-button"
            type="submit"
            disabled={busy || (!draft.trim() && !pictures.length && !recording)}
          >
            <FiSend /> {t("messages.send")}
          </button>
        </div>
      </form>

      {pictures.length ? (
        <div className="sf-thread-attachments">
          {pictures.map((picture, index) => (
            <span key={index} className="sf-thread-attachment">
              <img src={picture} alt="" />
              <button
                type="button"
                onClick={() => setPictures((current) => current.filter((_, i) => i !== index))}
                aria-label={t("common.delete")}
              >
                <FiX />
              </button>
            </span>
          ))}
        </div>
      ) : null}

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

      {deleting ? (
        <ConfirmModal
          title={t("messages.deleteTitle")}
          message={t("messages.deleteBody")}
          confirmLabel={t("common.delete")}
          onConfirm={async () => {
            try {
              await messagesApi.remove(deleting.id);
              // Reloaded rather than spliced out locally: the thread's unread
              // counts and the conversation list both move with it.
              await load();
              toast.success(t("messages.deleted"), toastOptions);
            } catch (err) {
              setError(err);
            } finally {
              setDeleting(null);
            }
          }}
          onClose={() => setDeleting(null)}
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
