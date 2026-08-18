import { useRef, useState } from "react";
import { toast } from "react-toastify";
import { FiImage, FiVideo, FiX } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { media, social } from "../../api/services";
import Avatar from "../common/Avatar";
import { ErrorMessage } from "../common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import { readImageAsDataUrl } from "../../utils/image";
import { firstUrl } from "../../utils/links";

const MAX_PICTURES = 4;

/**
 * Composes a post.
 *
 * Pictures are read into base64 data URIs and carried inline in the payload
 * (the API stores them in the JSONB column), so there is no upload step for
 * them. Videos are the opposite: too big for a row, so they go to the member's
 * own bucket and what lands in the post is the URL - typed into the text like
 * any other link.
 *
 * There is no separate link field. Whatever URL is in the text is the post's
 * link, which is why the preview below reacts to the textarea rather than to a
 * control of its own.
 */
export default function PostComposer({ clubs, defaultClubId, onPosted }) {
  const { t, tError } = useI18n();
  const { user, config } = useAuth();
  const fileInput = useRef(null);

  const videoInput = useRef(null);

  const [content, setContent] = useState("");
  const [pictures, setPictures] = useState([]);
  const [visibility, setVisibility] = useState(defaultClubId ? "club" : "public");
  const [clubId, setClubId] = useState(defaultClubId || "");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [uploading, setUploading] = useState(null);

  // What the post will actually embed, detected the same way the API detects
  // it - so the preview and the stored post cannot disagree.
  const link = firstUrl(content);

  const addPicture = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    try {
      const dataUrl = await readImageAsDataUrl(file, config?.maxImageSize);
      setPictures((current) => [...current, dataUrl].slice(0, MAX_PICTURES));
    } catch (err) {
      setError(err.message === "too-large" ? t("errors.imageTooLarge") : err);
    }
  };

  // Uploading a video appends its URL to the text rather than storing it
  // beside the post: from there it is an ordinary link, and the same detection
  // that handles a pasted YouTube URL renders it.
  const addVideo = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;

    setUploading(0);
    setError(null);
    try {
      const { url } = await media.uploadVideo(file, setUploading);
      setContent((current) => (current.trim() ? `${current.trim()}\n${url}` : url));
    } catch (err) {
      // The API's 405 for "no bucket configured" carries its own i18n code, so
      // it translates into "set up your storage first" through the same path
      // as every other failure - no status check needed here.
      toast.error(tError(err), toastOptions);
    } finally {
      setUploading(null);
    }
  };

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const post = await social.createPost({
        content,
        pictures,
        visibility,
        clubId: visibility === "club" ? clubId : "",
      });
      toast.success(t("feed.posted"), toastOptions);
      setContent("");
      setPictures([]);
      onPosted(post);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const canPost = Boolean(content.trim() || pictures.length) && (visibility !== "club" || clubId) && !uploading;

  return (
    <form className="sf-card" onSubmit={submit}>
      <div className="sf-row" style={{ alignItems: "flex-start", flexWrap: "nowrap" }}>
        <Avatar user={user} />
        <textarea
          className="sf-textarea"
          placeholder={t("feed.compose")}
          value={content}
          onChange={(event) => setContent(event.target.value)}
        />
      </div>

      {pictures.length > 0 ? (
        <div className="sf-post-media">
          {pictures.map((picture, index) => (
            <div key={index} style={{ position: "relative" }}>
              <img className="sf-post-picture" src={picture} alt="" />
              <button
                type="button"
                className="sf-button sf-button-ghost sf-button-sm"
                style={{ position: "absolute", top: 6, right: 6, background: "rgba(255,255,255,0.85)" }}
                onClick={() => setPictures((current) => current.filter((_, i) => i !== index))}
                aria-label={t("common.delete")}
              >
                <FiX />
              </button>
            </div>
          ))}
        </div>
      ) : null}

      {link ? (
        <div style={{ marginTop: "0.5rem" }}>
          <media-player url={link} />
        </div>
      ) : null}

      {uploading !== null ? (
        <p className="sf-muted" style={{ marginTop: "0.5rem" }}>
          {t("feed.uploadingVideo", { percent: uploading })}
        </p>
      ) : null}

      <ErrorMessage error={error} />

      <div className="sf-row-between" style={{ marginTop: "0.6rem" }}>
        <div className="sf-row" style={{ gap: "0.3rem" }}>
          <button
            type="button"
            className="sf-button-ghost sf-button-sm"
            onClick={() => fileInput.current?.click()}
            disabled={pictures.length >= MAX_PICTURES}
          >
            <FiImage /> {t("feed.addPicture")}
          </button>
          <input ref={fileInput} type="file" accept="image/*" hidden onChange={addPicture} />
          <button
            type="button"
            className="sf-button-ghost sf-button-sm"
            onClick={() => videoInput.current?.click()}
            disabled={uploading !== null}
          >
            <FiVideo /> {t("feed.addVideo")}
          </button>
          <input ref={videoInput} type="file" accept="video/*" hidden onChange={addVideo} />
        </div>

        <div className="sf-row" style={{ gap: "0.35rem" }}>
          <select
            className="sf-select sf-input-sm"
            style={{ width: "auto" }}
            value={visibility}
            onChange={(event) => setVisibility(event.target.value)}
            aria-label={t("feed.visibility")}
          >
            <option value="public">{t("feed.visibilityPublic")}</option>
            {(clubs || []).length > 0 ? <option value="club">{t("feed.visibilityClub")}</option> : null}
          </select>

          {visibility === "club" ? (
            <select
              className="sf-select sf-input-sm"
              style={{ width: "auto" }}
              value={clubId}
              onChange={(event) => setClubId(event.target.value)}
              aria-label={t("feed.pickClub")}
            >
              <option value="">{t("feed.pickClub")}</option>
              {(clubs || []).map((club) => (
                <option key={club.id} value={club.id}>
                  {club.name}
                </option>
              ))}
            </select>
          ) : null}

          <button className="sf-button sf-button-sm" type="submit" disabled={busy || !canPost}>
            {t("feed.post")}
          </button>
        </div>
      </div>
    </form>
  );
}
