import { useRef, useState } from "react";
import { toast } from "react-toastify";
import { FiImage, FiLink, FiX } from "react-icons/fi";

import { social } from "../../api/services";
import Avatar from "../common/Avatar";
import { ErrorMessage } from "../common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import { readImageAsDataUrl } from "../../utils/image";

const MAX_PICTURES = 4;
const MAX_LINKS = 4;

/**
 * Composes a post. Pictures are read into base64 data URIs and carried inline in
 * the post payload (the API stores them in the JSONB column), so there's no
 * separate upload step or object store to run.
 */
export default function PostComposer({ clubs, defaultClubId, onPosted }) {
  const { t } = useI18n();
  const { user, config } = useAuth();
  const fileInput = useRef(null);

  const [content, setContent] = useState("");
  const [pictures, setPictures] = useState([]);
  const [links, setLinks] = useState([]);
  const [linkDraft, setLinkDraft] = useState("");
  const [showLink, setShowLink] = useState(false);
  const [visibility, setVisibility] = useState(defaultClubId ? "club" : "public");
  const [clubId, setClubId] = useState(defaultClubId || "");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

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

  const addLink = () => {
    const url = linkDraft.trim();
    if (!url) return;
    setLinks((current) => [...current, url].slice(0, MAX_LINKS));
    setLinkDraft("");
    setShowLink(false);
  };

  const submit = async (event) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const post = await social.createPost({
        content,
        pictures,
        links,
        visibility,
        clubId: visibility === "club" ? clubId : "",
      });
      toast.success(t("feed.posted"));
      setContent("");
      setPictures([]);
      setLinks([]);
      onPosted(post);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const canPost = Boolean(content.trim() || pictures.length || links.length) && (visibility !== "club" || clubId);

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

      {links.map((link) => (
        <div key={link} className="sf-row" style={{ marginTop: "0.4rem" }}>
          <media-player url={link} />
          <button
            type="button"
            className="sf-button-ghost sf-button-sm"
            onClick={() => setLinks((current) => current.filter((item) => item !== link))}
            aria-label={t("common.delete")}
          >
            <FiX />
          </button>
        </div>
      ))}

      {showLink ? (
        <div className="sf-row" style={{ marginTop: "0.5rem", flexWrap: "nowrap" }}>
          <input
            className="sf-input sf-input-sm"
            placeholder={t("feed.linkPlaceholder")}
            value={linkDraft}
            autoFocus
            onChange={(event) => setLinkDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                addLink();
              }
            }}
          />
          <button type="button" className="sf-button sf-button-sm" onClick={addLink}>
            {t("common.add")}
          </button>
        </div>
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
            onClick={() => setShowLink((open) => !open)}
            disabled={links.length >= MAX_LINKS}
          >
            <FiLink /> {t("feed.addLink")}
          </button>
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
