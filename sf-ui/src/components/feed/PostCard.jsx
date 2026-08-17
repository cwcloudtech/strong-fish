import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import { FiEdit2, FiFlag, FiHeart, FiMessageSquare, FiTrash2 } from "react-icons/fi";

import { social } from "../../api/services";
import Avatar from "../common/Avatar";
import Modal, { ConfirmModal } from "../common/Modal";
import { ErrorMessage } from "../common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * One post in the feed. Links are rendered through the <media-player> custom
 * element (see webcomponents/MediaPlayer), so a pasted YouTube/Vimeo/Dailymotion
 * URL becomes a real player rather than a bare link.
 */
export default function PostCard({ post, onChanged, onDeleted }) {
  const { t, locale } = useI18n();
  const [comments, setComments] = useState(null);
  const [showComments, setShowComments] = useState(false);
  const [newComment, setNewComment] = useState("");
  const [editing, setEditing] = useState(false);
  const [editContent, setEditContent] = useState(post.content);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [reporting, setReporting] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!showComments || comments) return;
    social
      .comments(post.id)
      .then((page) => setComments(page.results))
      .catch(setError);
  }, [showComments, comments, post.id]);

  const toggleLike = async () => {
    try {
      onChanged(post.liked ? await social.unlike(post.id) : await social.like(post.id));
    } catch (err) {
      setError(err);
    }
  };

  const submitComment = async (event) => {
    event.preventDefault();
    if (!newComment.trim()) return;
    try {
      const comment = await social.addComment(post.id, newComment);
      setComments((current) => [...(current || []), comment]);
      setNewComment("");
      onChanged({ ...post, comments: post.comments + 1 });
    } catch (err) {
      setError(err);
    }
  };

  const removeComment = async (comment) => {
    try {
      await social.removeComment(post.id, comment.id);
      setComments((current) => current.filter((item) => item.id !== comment.id));
      onChanged({ ...post, comments: Math.max(0, post.comments - 1) });
      toast.success(t("feed.commentDeleted"));
    } catch (err) {
      setError(err);
    }
  };

  const saveEdit = async () => {
    try {
      onChanged(await social.updatePost(post.id, { content: editContent, pictures: post.pictures, links: post.links }));
      setEditing(false);
    } catch (err) {
      setError(err);
    }
  };

  const remove = async () => {
    setConfirmDelete(false);
    try {
      await social.removePost(post.id);
      toast.success(t("feed.postDeleted"));
      onDeleted(post.id);
    } catch (err) {
      setError(err);
    }
  };

  const authorPath = post.author?.handle ? `/profile/${post.author.handle}` : null;

  return (
    <div className="sf-card">
      <div className="sf-row">
        <Avatar user={post.author} />
        <div style={{ minWidth: 0 }}>
          <strong>
            {authorPath ? (
              <Link to={authorPath} style={{ color: "inherit" }}>
                {post.author.name} {post.author.surname}
              </Link>
            ) : (
              `${post.author.name} ${post.author.surname}`
            )}
          </strong>
          <div className="sf-muted">
            {new Date(post.createdAt).toLocaleString(locale)}
            {post.clubName ? ` · ${post.clubName}` : ""}
            {post.updatedAt !== post.createdAt ? ` · ${t("feed.edited")}` : ""}
          </div>
        </div>
        <span className="sf-spacer" />
        <span className="sf-badge sf-badge-muted">
          {post.visibility === "club" ? t("feed.visibilityClub") : t("feed.visibilityPublic")}
        </span>
      </div>

      {editing ? (
        <div style={{ marginTop: "0.7rem" }}>
          <textarea className="sf-textarea" value={editContent} onChange={(event) => setEditContent(event.target.value)} />
          <div className="sf-row" style={{ justifyContent: "flex-end", gap: "0.4rem", marginTop: "0.4rem" }}>
            <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => setEditing(false)}>
              {t("common.cancel")}
            </button>
            <button className="sf-button sf-button-sm" onClick={saveEdit}>
              {t("common.save")}
            </button>
          </div>
        </div>
      ) : (
        post.content && <p className="sf-post-content">{post.content}</p>
      )}

      {(post.pictures?.length || post.links?.length) ? (
        <div className="sf-post-media">
          {(post.pictures || []).map((picture, index) => (
            <img key={index} className="sf-post-picture" src={picture} alt="" />
          ))}
          {(post.links || []).map((link) => (
            <media-player key={link} url={link} />
          ))}
        </div>
      ) : null}

      <ErrorMessage error={error} />

      <div className="sf-post-actions">
        <button type="button" className="sf-button-ghost" onClick={toggleLike} style={{ color: post.liked ? "var(--sf-danger)" : undefined }}>
          <FiHeart /> {post.likes || 0}
        </button>
        <button type="button" className="sf-button-ghost" onClick={() => setShowComments((open) => !open)}>
          <FiMessageSquare /> {post.comments || 0}
        </button>
        <span className="sf-spacer" />
        {post.editable ? (
          <button type="button" className="sf-button-ghost" onClick={() => setEditing(true)} aria-label={t("common.edit")}>
            <FiEdit2 />
          </button>
        ) : null}
        {post.deletable ? (
          <button type="button" className="sf-button-ghost" onClick={() => setConfirmDelete(true)} aria-label={t("common.delete")}>
            <FiTrash2 />
          </button>
        ) : null}
        {!post.editable ? (
          <button type="button" className="sf-button-ghost" onClick={() => setReporting(true)} aria-label={t("feed.report")}>
            <FiFlag />
          </button>
        ) : null}
      </div>

      {showComments ? (
        <div>
          {(comments || []).map((comment) => (
            <div key={comment.id} className="sf-comment">
              <Avatar user={comment.author} size="sf-avatar-sm" />
              <div className="sf-comment-body">
                <strong>
                  {comment.author.name} {comment.author.surname}
                </strong>{" "}
                <span className="sf-muted">{new Date(comment.createdAt).toLocaleString(locale)}</span>
                <div>{comment.content}</div>
              </div>
              {comment.deletable ? (
                <button type="button" className="sf-button-ghost" onClick={() => removeComment(comment)} aria-label={t("common.delete")}>
                  <FiTrash2 />
                </button>
              ) : null}
            </div>
          ))}

          <form onSubmit={submitComment} className="sf-row" style={{ marginTop: "0.6rem", flexWrap: "nowrap" }}>
            <input
              className="sf-input sf-input-sm"
              placeholder={t("feed.writeComment")}
              value={newComment}
              onChange={(event) => setNewComment(event.target.value)}
            />
            <button className="sf-button sf-button-sm" type="submit" disabled={!newComment.trim()}>
              {t("feed.comment")}
            </button>
          </form>
        </div>
      ) : null}

      {confirmDelete ? (
        <ConfirmModal
          title={t("common.delete")}
          message={t("feed.confirmDeletePost")}
          onConfirm={remove}
          onClose={() => setConfirmDelete(false)}
        />
      ) : null}

      {reporting ? <ReportModal post={post} onClose={() => setReporting(false)} /> : null}
    </div>
  );
}

/** The "report this" dialog, filed for a superadmin to look at. */
function ReportModal({ post, onClose }) {
  const { t } = useI18n();
  const [reason, setReason] = useState("spam");
  const [comment, setComment] = useState("");
  const [error, setError] = useState(null);

  const submit = async () => {
    try {
      await social.report({ targetType: "post", targetId: post.id, reason, comment });
      toast.success(t("feed.reported"));
      onClose();
    } catch (err) {
      setError(err);
    }
  };

  return (
    <Modal
      title={t("feed.report")}
      onClose={onClose}
      actions={
        <>
          <button className="sf-button sf-button-secondary" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="sf-button sf-button-danger" onClick={submit}>
            {t("feed.report")}
          </button>
        </>
      }
    >
      <div className="sf-field">
        <label className="sf-label">{t("feed.reportReason")}</label>
        <select className="sf-select" value={reason} onChange={(event) => setReason(event.target.value)}>
          <option value="spam">{t("feed.reportSpam")}</option>
          <option value="abuse">{t("feed.reportAbuse")}</option>
          <option value="inappropriate">{t("feed.reportInappropriate")}</option>
          <option value="other">{t("feed.reportOther")}</option>
        </select>
      </div>
      <div className="sf-field">
        <label className="sf-label">{t("feed.reportComment")}</label>
        <textarea className="sf-textarea" value={comment} onChange={(event) => setComment(event.target.value)} />
      </div>
      <ErrorMessage error={error} />
    </Modal>
  );
}
