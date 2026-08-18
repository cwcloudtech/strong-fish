import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "react-toastify";
import { FiCheck, FiUsers, FiX } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import { invitations as invitationsApi } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The invitations waiting for this account.
 *
 * They are matched by email address rather than by user id, which is why an
 * invitation sent before somebody registered is sitting here when they arrive:
 * it was addressed to them before this app knew who they were.
 */
export default function Invitations() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [pending, setPending] = useState(null);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(null);

  const load = useCallback(async () => {
    try {
      setPending(await invitationsApi.mine());
    } catch (err) {
      setError(err);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (invitation, action) => {
    setBusy(invitation.id);
    try {
      if (action === "accept") {
        await invitationsApi.accept(invitation.id);
        toast.success(t("invitations.accepted", { club: invitation.clubName }), toastOptions);
        navigate(`/dashboard/clubs/${invitation.clubId}`);
        return;
      }
      await invitationsApi.decline(invitation.id);
      toast.success(t("invitations.declined"), toastOptions);
      await load();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="sf-page" style={{ maxWidth: 760 }}>
      <h1 className="sf-title">{t("invitations.title")}</h1>
      <p className="sf-subtitle">{t("invitations.subtitle")}</p>

      <ErrorMessage error={error} />

      {pending === null ? (
        <Spinner />
      ) : pending.length === 0 ? (
        <EmptyState title={t("invitations.emptyTitle")} message={t("invitations.emptyBody")} />
      ) : (
        pending.map((invitation) => (
          <div className="sf-card" key={invitation.id}>
            <h3 style={{ marginTop: 0, marginBottom: "0.2rem" }}>{invitation.clubName}</h3>
            <p className="sf-muted" style={{ margin: 0 }}>
              {t("invitations.from", { name: invitation.inviterName })}
              {invitation.memberCount ? (
                <>
                  {" · "}
                  <FiUsers style={{ verticalAlign: "-2px" }} />{" "}
                  {t("clubs.memberCount", { count: invitation.memberCount })}
                </>
              ) : null}
            </p>

            {invitation.message ? (
              <p style={{ whiteSpace: "pre-wrap", marginBottom: 0 }}>{invitation.message}</p>
            ) : null}

            <p className="sf-muted" style={{ fontSize: "0.85rem" }}>
              {t(`invitations.asRole.${invitation.role}`)}
            </p>

            <div className="sf-row" style={{ gap: "0.4rem" }}>
              <button
                className="sf-button"
                onClick={() => act(invitation, "accept")}
                disabled={busy === invitation.id}
              >
                <FiCheck /> {t("invitations.accept")}
              </button>
              <button
                className="sf-button sf-button-secondary"
                onClick={() => act(invitation, "decline")}
                disabled={busy === invitation.id}
              >
                <FiX /> {t("invitations.decline")}
              </button>
            </div>
          </div>
        ))
      )}
    </div>
  );
}
