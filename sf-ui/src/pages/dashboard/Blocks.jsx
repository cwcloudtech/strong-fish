import { useCallback, useEffect, useState } from "react";
import { toast } from "react-toastify";
import { FiUserCheck } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import Avatar from "../../components/common/Avatar";
import { blocks as blocksApi } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Who this member has blocked, and the way back.
 *
 * A block is reversible on purpose: it is a decision about a moment, and the
 * list exists so that decision can be revisited rather than being permanent by
 * accident.
 */
export default function Blocks() {
  const { t } = useI18n();
  const [blocked, setBlocked] = useState(null);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(null);

  const load = useCallback(async () => {
    try {
      setBlocked(await blocksApi.list());
    } catch (err) {
      setError(err);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const unblock = async (block) => {
    setBusy(block.blockedId);
    try {
      await blocksApi.unblock(block.blockedId);
      toast.success(t("blocks.unblocked"), toastOptions);
      await load();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="sf-page" style={{ maxWidth: 760 }}>
      <h1 className="sf-title">{t("blocks.title")}</h1>
      <p className="sf-subtitle">{t("blocks.subtitle")}</p>

      <ErrorMessage error={error} />

      {blocked === null ? (
        <Spinner />
      ) : blocked.length === 0 ? (
        <EmptyState title={t("blocks.emptyTitle")} message={t("blocks.emptyBody")} />
      ) : (
        <ul className="sf-list">
          {blocked.map((block) => (
            <li className="sf-list-item" key={block.id}>
              <Avatar user={block.blocked} size="sf-avatar-sm" />
              <div style={{ flex: 1, minWidth: 0 }}>
                <strong>
                  {block.blocked.name} {block.blocked.surname}
                </strong>
                <div className="sf-muted" style={{ fontSize: "0.85rem" }}>
                  {t("blocks.since", { date: new Date(block.createdAt).toLocaleDateString() })}
                </div>
              </div>
              <button
                className="sf-button sf-button-secondary sf-button-sm"
                onClick={() => unblock(block)}
                disabled={busy === block.blockedId}
              >
                <FiUserCheck /> {t("blocks.unblock")}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
