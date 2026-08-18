import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "react-toastify";
import { FiCalendar, FiEdit2, FiExternalLink, FiMapPin, FiPlus, FiTrash2 } from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import Tooltip from "../../components/common/Tooltip";
import { calendarFeed, clubs as clubsApi, events as eventsApi } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";

const KINDS = ["competition", "training", "other"];

/**
 * The calendar: meets, club sessions, camps.
 *
 * A member sees the open calendar plus their own clubs' dates; a coach can add
 * to the clubs they manage. Subscribing is the point of the feed URL at the
 * top - the calendar people actually live in is Outlook or Google's, not this
 * page.
 */
export default function Events() {
  const { t, locale } = useI18n();
  const { user } = useAuth();
  const [events, setEvents] = useState(null);
  const [clubs, setClubs] = useState([]);
  const [error, setError] = useState(null);
  const [showPast, setShowPast] = useState(false);
  const [editing, setEditing] = useState(null);
  const [deleting, setDeleting] = useState(null);

  const load = useCallback(async () => {
    try {
      const [list, myClubs] = await Promise.all([
        eventsApi.list(showPast ? { past: 1 } : undefined),
        clubsApi.list().catch(() => []),
      ]);
      setEvents(list);
      setClubs(myClubs);
    } catch (err) {
      setError(err);
    }
  }, [showPast]);

  useEffect(() => {
    load();
  }, [load]);

  // Only the clubs this member manages can receive an event; a superadmin can
  // also put one on the open calendar, which is what the blank option is.
  const writableClubs = useMemo(
    () => clubs.filter((club) => club.role === "owner" || club.role === "admin"),
    [clubs]
  );
  const canCreate = writableClubs.length > 0 || user?.role === "superadmin";

  const remove = async () => {
    try {
      await eventsApi.remove(deleting.id);
      toast.success(t("events.deleted"), toastOptions);
      await load();
    } catch (err) {
      setError(err);
    } finally {
      setDeleting(null);
    }
  };

  return (
    <div className="sf-page" style={{ maxWidth: 900 }}>
      <div className="sf-page-header">
        <div>
          <h1 className="sf-title">{t("events.title")}</h1>
          <p className="sf-subtitle">{t("events.subtitle")}</p>
        </div>
        {canCreate ? (
          <button className="sf-button" onClick={() => setEditing({})}>
            <FiPlus /> {t("events.add")}
          </button>
        ) : null}
      </div>

      <CalendarSubscription />

      <ErrorMessage error={error} />

      <label className="sf-row" style={{ gap: "0.4rem", alignItems: "center", margin: "0.6rem 0" }}>
        <input type="checkbox" checked={showPast} onChange={(event) => setShowPast(event.target.checked)} />
        {t("events.showPast")}
      </label>

      {events === null ? (
        <Spinner />
      ) : events.length === 0 ? (
        <EmptyState title={t("events.emptyTitle")} message={t("events.emptyBody")} />
      ) : (
        events.map((event) => (
          <EventCard
            key={event.id}
            event={event}
            locale={locale}
            t={t}
            onEdit={() => setEditing(event)}
            onDelete={() => setDeleting(event)}
          />
        ))
      )}

      {editing ? (
        <EventFormModal
          event={editing}
          clubs={writableClubs}
          canPostGlobally={user?.role === "superadmin"}
          onClose={() => setEditing(null)}
          onSaved={async () => {
            setEditing(null);
            await load();
          }}
        />
      ) : null}

      {deleting ? (
        <ConfirmModal
          title={t("events.deleteTitle")}
          message={t("events.deleteBody", { title: deleting.title })}
          confirmLabel={t("common.delete")}
          onConfirm={remove}
          onClose={() => setDeleting(null)}
        />
      ) : null}
    </div>
  );
}

/** One entry. */
function EventCard({ event, locale, t, onEdit, onDelete }) {
  const start = new Date(event.startsAt);
  const end = event.endsAt ? new Date(event.endsAt) : null;

  const dateLabel = event.allDay
    ? start.toLocaleDateString(locale, { weekday: "short", day: "numeric", month: "long", year: "numeric" })
    : start.toLocaleString(locale, {
        weekday: "short",
        day: "numeric",
        month: "long",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });

  return (
    <div className="sf-card">
      <div className="sf-row-between" style={{ alignItems: "flex-start" }}>
        <div style={{ minWidth: 0 }}>
          <span className={`sf-badge sf-badge-${event.kind}`}>{t(`events.kind.${event.kind}`)}</span>
          <h3 style={{ margin: "0.4rem 0 0.2rem" }}>{event.title}</h3>
          <p className="sf-muted" style={{ margin: 0 }}>
            <FiCalendar style={{ verticalAlign: "-2px" }} /> {dateLabel}
            {end && !event.allDay ? ` – ${end.toLocaleTimeString(locale, { hour: "2-digit", minute: "2-digit" })}` : ""}
          </p>
          {event.location ? (
            <p className="sf-muted" style={{ margin: "0.2rem 0 0" }}>
              <FiMapPin style={{ verticalAlign: "-2px" }} /> {event.location}
            </p>
          ) : null}
          {event.clubName ? (
            <p className="sf-muted" style={{ margin: "0.2rem 0 0" }}>
              {event.clubName}
            </p>
          ) : null}
        </div>

        {event.editable ? (
          <div className="sf-row" style={{ gap: "0.25rem" }}>
            <Tooltip label={t("common.edit")}>
              <button className="sf-icon-button sf-icon-button-plain" onClick={onEdit} aria-label={t("common.edit")}>
                <FiEdit2 />
              </button>
            </Tooltip>
            <Tooltip label={t("common.delete")}>
              <button className="sf-icon-button sf-icon-button-plain" onClick={onDelete} aria-label={t("common.delete")}>
                <FiTrash2 />
              </button>
            </Tooltip>
          </div>
        ) : null}
      </div>

      {event.description ? <p style={{ whiteSpace: "pre-wrap" }}>{event.description}</p> : null}

      {event.url ? (
        <a href={event.url} target="_blank" rel="noopener noreferrer" className="sf-row" style={{ gap: "0.3rem" }}>
          <FiExternalLink /> {t("events.moreInfo")}
        </a>
      ) : null}
    </div>
  );
}

/**
 * Creates or edits one event.
 *
 * Dates are edited as the browser's own local date/time and converted to an
 * instant on save - a meet starts at a wall-clock hour where it is held, and
 * making the coach do the timezone arithmetic would be a good way to get it
 * wrong.
 */
function EventFormModal({ event, clubs, canPostGlobally, onClose, onSaved }) {
  const { t } = useI18n();
  const [form, setForm] = useState(() => ({
    clubId: event.clubId || (clubs.length === 1 && !canPostGlobally ? clubs[0].id : ""),
    title: event.title || "",
    description: event.description || "",
    location: event.location || "",
    url: event.url || "",
    kind: event.kind || "competition",
    allDay: Boolean(event.allDay),
    startsAt: toInputValue(event.startsAt, event.allDay),
    endsAt: toInputValue(event.endsAt, event.allDay),
    visibility: event.visibility || "public",
  }));
  const [busy, setBusy] = useState(false);

  const set = (field) => (fieldEvent) => {
    const value = fieldEvent.target.type === "checkbox" ? fieldEvent.target.checked : fieldEvent.target.value;
    setForm((current) => ({ ...current, [field]: value }));
  };

  const submit = async (formEvent) => {
    formEvent.preventDefault();
    setBusy(true);
    try {
      const payload = {
        ...form,
        startsAt: toInstant(form.startsAt, form.allDay),
        endsAt: form.endsAt ? toInstant(form.endsAt, form.allDay) : "",
      };
      if (event.id) {
        await eventsApi.update(event.id, payload);
      } else {
        await eventsApi.create(payload);
      }
      toast.success(t(event.id ? "events.updated" : "events.created"), toastOptions);
      await onSaved();
    } catch (err) {
      toast.error(err?.response?.data?.message || t("errors.generic"), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      title={t(event.id ? "events.edit" : "events.add")}
      onClose={onClose}
      actions={
        <>
          <button type="button" className="sf-button sf-button-secondary" onClick={onClose} disabled={busy}>
            {t("common.cancel")}
          </button>
          <button type="submit" form="sf-event-form" className="sf-button" disabled={busy || !form.title || !form.startsAt}>
            {busy ? t("common.loading") : t("common.save")}
          </button>
        </>
      }
    >
      <form id="sf-event-form" onSubmit={submit}>
        <div className="sf-field">
          <label className="sf-label" htmlFor="title">
            {t("events.eventTitle")}
          </label>
          <input id="title" className="sf-input" value={form.title} onChange={set("title")} required />
        </div>

        <div className="sf-row" style={{ gap: "0.6rem" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
            <label className="sf-label" htmlFor="kind">
              {t("events.kindLabel")}
            </label>
            <select id="kind" className="sf-select" value={form.kind} onChange={set("kind")}>
              {KINDS.map((kind) => (
                <option key={kind} value={kind}>
                  {t(`events.kind.${kind}`)}
                </option>
              ))}
            </select>
          </div>

          {/* The club is fixed once an event exists: moving it would change who
              can see it after the fact. */}
          {!event.id ? (
            <div className="sf-field" style={{ flex: 1, minWidth: 150 }}>
              <label className="sf-label" htmlFor="clubId">
                {t("events.club")}
              </label>
              <select id="clubId" className="sf-select" value={form.clubId} onChange={set("clubId")}>
                {canPostGlobally ? <option value="">{t("events.noClub")}</option> : null}
                {clubs.map((club) => (
                  <option key={club.id} value={club.id}>
                    {club.name}
                  </option>
                ))}
              </select>
            </div>
          ) : null}
        </div>

        <label className="sf-row" style={{ gap: "0.4rem", alignItems: "center", margin: "0.4rem 0" }}>
          <input type="checkbox" checked={form.allDay} onChange={set("allDay")} />
          {t("events.allDay")}
        </label>

        <div className="sf-row" style={{ gap: "0.6rem" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 170 }}>
            <label className="sf-label" htmlFor="startsAt">
              {t("events.startsAt")}
            </label>
            <input
              id="startsAt"
              className="sf-input"
              type={form.allDay ? "date" : "datetime-local"}
              value={form.startsAt}
              onChange={set("startsAt")}
              required
            />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 170 }}>
            <label className="sf-label" htmlFor="endsAt">
              {t("events.endsAt")} <span className="sf-muted">({t("common.optional")})</span>
            </label>
            <input
              id="endsAt"
              className="sf-input"
              type={form.allDay ? "date" : "datetime-local"}
              value={form.endsAt}
              onChange={set("endsAt")}
            />
          </div>
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="location">
            {t("events.location")} <span className="sf-muted">({t("common.optional")})</span>
          </label>
          <input id="location" className="sf-input" value={form.location} onChange={set("location")} />
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="url">
            {t("events.url")} <span className="sf-muted">({t("common.optional")})</span>
          </label>
          <input id="url" className="sf-input" type="url" value={form.url} onChange={set("url")} />
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="description">
            {t("events.description")} <span className="sf-muted">({t("common.optional")})</span>
          </label>
          <textarea id="description" className="sf-textarea" rows={4} value={form.description} onChange={set("description")} />
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="visibility">
            {t("events.visibility")}
          </label>
          <select id="visibility" className="sf-select" value={form.visibility} onChange={set("visibility")}>
            <option value="public">{t("events.visibilityPublic")}</option>
            <option value="club">{t("events.visibilityClub")}</option>
          </select>
        </div>
      </form>
    </Modal>
  );
}

/** Subscribe-from-Outlook/Google panel: the feed URL, and how to revoke it. */
function CalendarSubscription() {
  const { t } = useI18n();
  const [status, setStatus] = useState(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setStatus(await calendarFeed.status());
    } catch {
      // The panel is an extra; a failure here must not take the page with it.
      setStatus({ enabled: false });
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (action) => {
    setBusy(true);
    try {
      setStatus(await calendarFeed[action]());
      if (action === "regenerate") toast.success(t("events.feedRegenerated"), toastOptions);
    } catch {
      toast.error(t("errors.generic"), toastOptions);
    } finally {
      setBusy(false);
    }
  };

  const copy = () => {
    navigator.clipboard
      ?.writeText(status.url)
      .then(() => toast.success(t("common.copied"), toastOptions))
      .catch(() => toast.error(t("errors.copyFailed"), toastOptions));
  };

  if (!status) return null;

  return (
    <div className="sf-card">
      <h3 style={{ marginTop: 0 }}>{t("events.subscribeTitle")}</h3>
      <p className="sf-muted" style={{ marginTop: 0 }}>
        {t("events.subscribeBody")}
      </p>

      {status.enabled && status.url ? (
        <>
          <div className="sf-token-box">
            <code className="sf-token">{status.url}</code>
          </div>
          <div className="sf-row" style={{ gap: "0.4rem", marginTop: "0.6rem" }}>
            <button className="sf-button sf-button-secondary sf-button-sm" onClick={copy}>
              {t("events.copyFeed")}
            </button>
            <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => act("regenerate")} disabled={busy}>
              {t("events.regenerateFeed")}
            </button>
            <button className="sf-button sf-button-secondary sf-button-sm" onClick={() => act("disable")} disabled={busy}>
              {t("events.disableFeed")}
            </button>
          </div>
          <p className="sf-muted" style={{ fontSize: "0.82rem", marginBottom: 0 }}>
            {t("events.feedWarning")}
          </p>
        </>
      ) : (
        <button className="sf-button sf-button-sm" onClick={() => act("enable")} disabled={busy}>
          {t("events.enableFeed")}
        </button>
      )}
    </div>
  );
}

/**
 * An instant, rendered for a `date` or `datetime-local` input - both of which
 * speak local wall-clock time with no zone, so the conversion has to go through
 * the browser's own offset rather than through toISOString.
 */
function toInputValue(value, allDay) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (n) => String(n).padStart(2, "0");
  const day = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  return allDay ? day : `${day}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** The inverse: a local input value back to an RFC 3339 instant. */
function toInstant(value, allDay) {
  if (!value) return "";
  const date = new Date(allDay ? `${value}T00:00` : value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}
