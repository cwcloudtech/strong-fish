import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { toast } from "react-toastify";
import { FiEdit2, FiFlag, FiHeart, FiMessageSquare, FiShare2, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { clubs as clubsApi, social } from "../../api/services";
import Avatar from "../common/Avatar";
import Modal, { ConfirmModal } from "../common/Modal";
import ShareButtons from "../common/ShareButtons";
import { postShareUrl, shareTextFor } from "../../utils/shareText";
import { ErrorMessage } from "../common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import LinkifiedText from "../common/LinkifiedText";
import Select from "../common/Select";
import MultiSelect from "../common/MultiSelect";
import MediaLink from "../common/MediaLink";

/**
 * One post in the feed. Links are rendered through the <media-player> custom
 * element (see webcomponents/MediaPlayer), so a pasted YouTube/Vimeo/Dailymotion
 * URL becomes a real player rather than a bare link.
 */
export default function PostCard({ post, onChanged, onDeleted }) {
  const { t, locale } = useI18n();
  const { user } = useAuth();
  const [comments, setComments] = useState(null);
  const [showComments, setShowComments] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [newComment, setNewComment] = useState("");
  // The comment being edited, and the text as it is being retyped.
  const [editingComment, setEditingComment] = useState(null);
  const [commentDraft, setCommentDraft] = useState("");
  const [editing, setEditing] = useState(false);
  const [editContent, setEditContent] = useState(post.content);
  const [editVisibility, setEditVisibility] = useState(post.visibility);
  const [editClubIds, setEditClubIds] = useState(post.clubIds || []);
  // The author's own clubs, for moving the post between them. Loaded when the
  // editor opens rather than with the card: a feed renders dozens of these and
  // almost none of them are ever edited.
  const [myClubs, setMyClubs] = useState(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [reporting, setReporting] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!editing || myClubs) return;
    clubsApi
      .list()
      .then((list) => setMyClubs(list || []))
      .catch(() => setMyClubs([]));
  }, [editing, myClubs]);

  useEffect(() => {
    if (!showComments || comments) return;
    social
      .comments(post.id)
      .then((page) => setComments(page?.results || []))
      .catch(setError);
  }, [showComments, comments, post.id]);

  // The post's own author, which decides whether the like control is offered.
  // Not post.editable: a superadmin can edit anybody's post and is still free
  // to like it.
  const isAuthor = user?.id === post.author?.id;

  // The clubs this post may be moved into. The author picks from their own; a
  // superadmin moderating somebody else's gets only the club it is already in,
  // because publishing an author into a club they never joined is not a
  // moderation power - the API rejects it, and the UI should not offer it.
  const movableClubs = isAuthor
    ? myClubs || []
    : (post.clubIds || []).map((id, index) => ({
        id,
        name: post.clubNames?.[index] || t("feed.visibilityClub"),
      }));

  const toggleLike = async () => {
    try {
      onChanged(post.liked ? await social.unlike(post.id) : await social.like(post.id));
    } catch (err) {
      setError(err);
    }
  };

  const saveComment = async (event) => {
    event.preventDefault();
    const content = commentDraft.trim();
    if (!content) return;
    try {
      const updated = await social.updateComment(post.id, editingComment.id, content);
      setComments((current) =>
        (current || []).map((item) => (item.id === updated.id ? updated : item))
      );
      setEditingComment(null);
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
      toast.success(t("feed.commentDeleted"), toastOptions);
    } catch (err) {
      setError(err);
    }
  };

  const startEditing = () => {
    setEditContent(post.content);
    setEditVisibility(post.visibility);
    setEditClubIds(post.clubIds || []);
    setEditing(true);
  };

  const saveEdit = async () => {
    try {
      // No links are sent: the API re-derives them from the edited text, so
      // fixing a typo'd URL fixes the embed instead of leaving a stale one.
      onChanged(
        await social.updatePost(post.id, {
          content: editContent,
          pictures: post.pictures,
          visibility: editVisibility,
          clubIds: editVisibility === "club" ? editClubIds : [],
        })
      );
      setEditing(false);
    } catch (err) {
      setError(err);
    }
  };

  const remove = async () => {
    setConfirmDelete(false);
    try {
      await social.removePost(post.id);
      toast.success(t("feed.postDeleted"), toastOptions);
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
            {/* Every club it was shared with, not just the first: a post in
                two clubs says so. */}
            {post.clubNames?.length ? ` · ${post.clubNames.join(", ")}` : ""}
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

          {/* Where the post is published, changeable by its author or by a
              superadmin - who can take a post off a club feed, but only ever
              into a club its author already belongs to. Their own club list is
              not the author's, so a non-author is offered the post's current
              club and nothing else; the API enforces the same rule. */}
          <div className="sf-row" style={{ gap: "0.35rem", marginTop: "0.5rem" }}>
            <Select
              className="sf-select-inline"
              options={[
                { value: "public", label: t("feed.visibilityPublic") },
                ...(movableClubs.length > 0 ? [{ value: "club", label: t("feed.visibilityClub") }] : []),
              ]}
              value={editVisibility}
              onChange={(next) => {
                setEditVisibility(next);
                // Moving to a club with only one to choose from should not
                // need a second click.
                if (next === "club" && editClubIds.length === 0 && movableClubs.length === 1) {
                  setEditClubIds([movableClubs[0].id]);
                }
              }}
              placeholder={t("feed.visibility")}
            />

            {editVisibility === "club" ? (
              <MultiSelect
                className="sf-select-inline"
                options={movableClubs.map((club) => ({ value: club.id, label: club.name }))}
                selected={editClubIds}
                onChange={setEditClubIds}
                placeholder={t("feed.pickClubs")}
              />
            ) : null}
          </div>

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
        post.content && (
          <p className="sf-post-content">
            <LinkifiedText text={post.content} />
          </p>
        )
      )}

      {(post.pictures?.length || post.links?.length) ? (
        <div className="sf-post-media">
          {(post.pictures || []).map((picture, index) => (
            <img key={index} className="sf-post-picture" src={picture} alt="" />
          ))}
          {(post.links || []).map((link) => (
            <MediaLink key={link} url={link} />
          ))}
        </div>
      ) : null}

      <ErrorMessage error={error} />

      <div className="sf-post-actions">
        {/* The author sees the count but cannot add to it: a like says how a
            post landed with other people, which is not something its writer can
            contribute. The API refuses it too - this only keeps a dead control
            off the screen. */}
        {isAuthor ? (
          <span className="sf-post-likes" title={t("feed.ownPostLikes")}>
            <FiHeart /> {post.likes || 0}
          </span>
        ) : (
          <button
            type="button"
            className="sf-button-ghost"
            onClick={toggleLike}
            style={{ color: post.liked ? "var(--sf-danger)" : undefined }}
          >
            <FiHeart /> {post.likes || 0}
          </button>
        )}
        <button type="button" className="sf-button-ghost" onClick={() => setShowComments((open) => !open)}>
          <FiMessageSquare /> {post.comments || 0}
        </button>
        {/* Only a public post can be shared. A club-only post's link would
            answer 404 to whoever opened it, so offering the button would be
            offering something that cannot work. */}
        {post.visibility === "public" ? (
          <button
            type="button"
            className="sf-button-ghost"
            onClick={() => setSharing((open) => !open)}
            aria-label={t("share.label")}
            aria-expanded={sharing}
          >
            <FiShare2 />
          </button>
        ) : null}
        <span className="sf-spacer" />
        {post.editable ? (
          <button type="button" className="sf-button-ghost" onClick={startEditing} aria-label={t("common.edit")}>
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

      {/* Folded away until asked for: six icons under every post in the feed
          would outweigh the post. */}
      {sharing ? (
        <ShareButtons
          // The post itself, not its author's profile: somebody following a
          // shared link is coming for the thing that was shared.
          url={postShareUrl(post.id)}
          // The links come out of the text - one of them is the post's own
          // embed, and sending a reader to that instead of to the post is
          // exactly what sharing should not do.
          text={shareTextFor(post.content, t("share.postText"))}
        />
      ) : null}

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
                {/* Says so when it has been changed since. A comment somebody
                    replied to may not be the one on screen any more, and the
                    thread should not hide that. */}
                {wasEdited(comment) ? (
                  <span className="sf-muted" title={new Date(comment.updatedAt).toLocaleString(locale)}>
                    {" · "}
                    {t("feed.editedAt", { date: new Date(comment.updatedAt).toLocaleString(locale) })}
                  </span>
                ) : null}

                {editingComment?.id === comment.id ? (
                  <form onSubmit={saveComment} className="sf-row" style={{ marginTop: "0.3rem", flexWrap: "nowrap" }}>
                    <input
                      className="sf-input sf-input-sm"
                      value={commentDraft}
                      autoFocus
                      onChange={(event) => setCommentDraft(event.target.value)}
                      onKeyDown={(event) => event.key === "Escape" && setEditingComment(null)}
                    />
                    <button className="sf-button sf-button-sm" type="submit" disabled={!commentDraft.trim()}>
                      {t("common.save")}
                    </button>
                    <button
                      type="button"
                      className="sf-button-ghost sf-button-sm"
                      onClick={() => setEditingComment(null)}
                    >
                      {t("common.cancel")}
                    </button>
                  </form>
                ) : (
                  <div>
                    <LinkifiedText text={comment.content} />
                  </div>
                )}
              </div>

              {comment.editable && editingComment?.id !== comment.id ? (
                <button
                  type="button"
                  className="sf-button-ghost"
                  onClick={() => {
                    setEditingComment(comment);
                    setCommentDraft(comment.content);
                  }}
                  aria-label={t("common.edit")}
                >
                  <FiEdit2 />
                </button>
              ) : null}
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
      toast.success(t("feed.reported"), toastOptions);
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
        <Select
          options={[
            { value: "spam", label: t("feed.reportSpam") },
            { value: "abuse", label: t("feed.reportAbuse") },
            { value: "inappropriate", label: t("feed.reportInappropriate") },
            { value: "other", label: t("feed.reportOther") },
          ]}
          value={reason}
          onChange={setReason}
        />
      </div>
      <div className="sf-field">
        <label className="sf-label">{t("feed.reportComment")}</label>
        <textarea className="sf-textarea" value={comment} onChange={(event) => setComment(event.target.value)} />
      </div>
      <ErrorMessage error={error} />
    </Modal>
  );
}

/**
 * Whether a comment has been changed since it was written.
 *
 * Strict, and it can be: both timestamps default to now(), which Postgres holds
 * fixed for the whole transaction, so a freshly written row carries two
 * identical stamps. This is the same comparison the post above it uses - the
 * rule should not differ between a post and a comment.
 */
function wasEdited(comment) {
  return Boolean(comment.updatedAt) && comment.updatedAt !== comment.createdAt;
}
