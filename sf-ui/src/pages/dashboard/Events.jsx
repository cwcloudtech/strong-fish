import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "react-toastify";
import {
  FiCalendar,
  FiEdit2,
  FiExternalLink,
  FiGrid,
  FiList,
  FiMapPin,
  FiPlus,
  FiTrash2,
  FiUpload,
} from "react-icons/fi";

import toastOptions from "../../utils/toastOptions";
import Modal, { ConfirmModal } from "../../components/common/Modal";
import Tooltip from "../../components/common/Tooltip";
import EventCalendar from "../../components/calendar/EventCalendar";
import { calendarFeed, clubs as clubsApi, events as eventsApi } from "../../api/services";
import { EmptyState, ErrorMessage, Spinner } from "../../components/common/Feedback";
import { useAuth } from "../../context/AuthContext";
import { useI18n } from "../../i18n/I18nContext";
import Switch from "../../components/common/Switch";
import isWholeDay from "../../utils/wholeDay";
import LinkifiedText from "../../components/common/LinkifiedText";

const KINDS = ["competition", "training", "other"];

const LAYOUT_KEY = "sf.eventsLayout";

/**
 * The palette a new event's colour is drawn from.
 *
 * Pre-selected at random rather than defaulting to one: a calendar where every
 * event is the same colour tells you nothing, and nobody picks a colour for the
 * first entry if the form already looks filled in. Picking randomly means a
 * month of events is legible without anybody having thought about it.
 */
const EVENT_COLORS = [
  "#0e5e9b", "#2b8fd4", "#16a34a", "#65a30d", "#d97706",
  "#dc2626", "#db2777", "#7c3aed", "#0891b2", "#4b5563",
];

const randomColor = () => EVENT_COLORS[Math.floor(Math.random() * EVENT_COLORS.length)];

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
  const [layout, setLayout] = useState(() => localStorage.getItem(LAYOUT_KEY) || "calendar");
  const [view, setView] = useState("month");
  const [anchor, setAnchor] = useState(() => new Date());
  // The grid shows a month at a time, including the part of it already gone,
  // so it always asks for the past - the list keeps its own toggle.
  const [selected, setSelected] = useState(null);
  // Whether the grid has already been pointed at the events. Done once, on the
  // first load: after that the month shown is wherever the reader navigated to,
  // and moving it under them would be worse than showing an empty month.
  const anchored = useRef(false);

  const load = useCallback(async () => {
    try {
      // A grid always shows days that have already happened, so it asks for
      // the whole calendar; the list is forward-looking unless asked.
      const [list, myClubs] = await Promise.all([
        eventsApi.list(showPast || layout === "calendar" ? { past: 1 } : undefined),
        clubsApi.list().catch(() => []),
      ]);
      setEvents(list);
      setClubs(myClubs);
      anchorOnEvents(list);
    } catch (err) {
      setError(err);
    }
  }, [showPast, layout]);

  useEffect(() => {
    load();
  }, [load]);

  /**
   * Opens the grid where the events are.
   *
   * A calendar that opens on today shows an empty month to anybody whose next
   * meet is in October, which reads as "the calendar is broken" rather than as
   * "nothing this month". So if the current month holds nothing, it jumps to
   * the nearest event instead - forward by preference, since a calendar is
   * about what is coming.
   */
  const anchorOnEvents = (list) => {
    if (anchored.current || !list?.length) return;
    anchored.current = true;

    const now = new Date();
    const inThisMonth = list.some((event) => {
      const at = new Date(event.startsAt);
      return (
        !Number.isNaN(at.getTime()) &&
        at.getFullYear() === now.getFullYear() &&
        at.getMonth() === now.getMonth()
      );
    });
    if (inThisMonth) return;

    const dated = list
      .map((event) => new Date(event.startsAt))
      .filter((at) => !Number.isNaN(at.getTime()))
      .sort((a, b) => a - b);
    if (!dated.length) return;

    const upcoming = dated.find((at) => at >= now);
    setAnchor(upcoming || dated[dated.length - 1]);
  };

  const setLayoutAndRemember = (next) => {
    setLayout(next);
    localStorage.setItem(LAYOUT_KEY, next);
  };

  // Only the clubs this member manages can receive an event; a superadmin can
  // also put one on the open calendar, which is what the blank option is.
  const writableClubs = useMemo(
    () => clubs.filter((club) => club.role === "owner" || club.role === "admin"),
    [clubs]
  );
  // Anybody signed in can add to their own calendar - the form limits what
  // they may publish, not whether they may write anything at all.
  const canPublish = writableClubs.length > 0 || user?.role === "superadmin";
  const canCreate = Boolean(user);

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
        <div className="sf-row" style={{ gap: "0.4rem" }}>
          {/* Importing publishes a whole season at once, so it is offered to
              the same people who may publish one event: a club's coaches and
              a superadmin. The API checks it again. */}
          {canPublish ? <CalendarImport clubs={writableClubs} canPostGlobally={user?.role === "superadmin"} onImported={load} /> : null}
          {canCreate ? (
            <button className="sf-button" onClick={() => setEditing({})}>
              <FiPlus /> {t("events.add")}
            </button>
          ) : null}
        </div>
      </div>

      <CalendarSubscription />

      <ErrorMessage error={error} />

      <div className="sf-row-between" style={{ margin: "0.6rem 0", flexWrap: "wrap", gap: "0.5rem" }}>
        <div className="sf-calendar-views">
          <Tooltip label={t("events.calendarLayout")}>
            <button
              type="button"
              className={`sf-calendar-view ${layout === "calendar" ? "active" : ""}`}
              onClick={() => setLayoutAndRemember("calendar")}
              aria-label={t("events.calendarLayout")}
            >
              <FiGrid style={{ verticalAlign: "-2px" }} />
            </button>
          </Tooltip>
          <Tooltip label={t("events.listLayout")}>
            <button
              type="button"
              className={`sf-calendar-view ${layout === "list" ? "active" : ""}`}
              onClick={() => setLayoutAndRemember("list")}
              aria-label={t("events.listLayout")}
            >
              <FiList style={{ verticalAlign: "-2px" }} />
            </button>
          </Tooltip>
        </div>

        {layout === "list" ? (
          <label className="sf-row" style={{ gap: "0.4rem", alignItems: "center" }}>
            <input type="checkbox" checked={showPast} onChange={(event) => setShowPast(event.target.checked)} />
            {t("events.showPast")}
          </label>
        ) : null}
      </div>

      {events === null ? (
        <Spinner />
      ) : layout === "calendar" ? (
        <div className="sf-calendar-scroll">
          <EventCalendar
            events={events}
            anchor={anchor}
            view={view}
            onAnchorChange={setAnchor}
            onViewChange={setView}
            onSelect={setSelected}
            // Starting an event from a day pre-fills that date, which is the
            // whole reason to click a day rather than the "add" button.
            onAddOn={(date) => setEditing({ startsOn: date })}
            // From the week view: the exact range the coach dragged, in
            // minutes past midnight.
            onAddRange={(date, fromMinutes, toMinutes) =>
              setEditing({ startsOn: date, fromMinutes, toMinutes })
            }
            canCreate={canCreate}
          />
        </div>
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

      {selected ? (
        <EventDetailModal
          event={selected}
          locale={locale}
          t={t}
          onClose={() => setSelected(null)}
          onEdit={() => {
            setEditing(selected);
            setSelected(null);
          }}
          onDelete={() => {
            setDeleting(selected);
            setSelected(null);
          }}
        />
      ) : null}

      {editing ? (
        <EventFormModal
          event={editing}
          clubs={writableClubs}
          canPostGlobally={user?.role === "superadmin"}
          canPublish={canPublish}
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

  // Birthdays are the one entry that occupies a day rather than a time of day,
  // and they are generated rather than authored - everything somebody creates
  // happens at a stated hour.
  const wholeDay = isWholeDay(event);
  const dateLabel = wholeDay
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
          {/* The agenda is the same events as the grids, so it carries the
              same colour - otherwise switching layout loses the cue. */}
          {event.color ? (
            <span className="sf-color-dot" style={{ backgroundColor: event.color }} aria-hidden="true" />
          ) : null}
          <span className={`sf-badge sf-badge-${event.kind}`}>{t(`events.kind.${event.kind}`)}</span>
          <h3 style={{ margin: "0.4rem 0 0.2rem" }}>{event.title}</h3>
          <p className="sf-muted" style={{ margin: 0 }}>
            <FiCalendar style={{ verticalAlign: "-2px" }} /> {dateLabel}
            {end && !wholeDay ? ` – ${end.toLocaleTimeString(locale, { hour: "2-digit", minute: "2-digit" })}` : ""}
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

      {event.description ? (
        <p style={{ whiteSpace: "pre-wrap" }}>
          <LinkifiedText text={event.description} />
        </p>
      ) : null}

      {event.url ? (
        <a href={event.url} target="_blank" rel="noopener noreferrer" className="sf-row" style={{ gap: "0.3rem" }}>
          <FiExternalLink /> {t("events.moreInfo")}
        </a>
      ) : null}
    </div>
  );
}

/**
 * One event, opened from the grid.
 *
 * The grid's chips are small and say only the time and the title, so selecting
 * one has to be able to show the rest - and it is where editing and deleting
 * are reached from, since a chip has no room for its own buttons.
 */
function EventDetailModal({ event, locale, t, onClose, onEdit, onDelete }) {
  const start = new Date(event.startsAt);
  const end = event.endsAt ? new Date(event.endsAt) : null;
  const wholeDay = isWholeDay(event);

  const when = wholeDay
    ? start.toLocaleDateString(locale, { weekday: "long", day: "numeric", month: "long", year: "numeric" })
    : start.toLocaleString(locale, {
        weekday: "long",
        day: "numeric",
        month: "long",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });

  return (
    <Modal title={event.title} onClose={onClose}>
      <span className={`sf-badge sf-badge-${event.kind}`}>{t(`events.kind.${event.kind}`)}</span>

      <p className="sf-muted" style={{ margin: "0.6rem 0 0" }}>
        <FiCalendar style={{ verticalAlign: "-2px" }} /> {when}
        {end && !wholeDay
          ? ` – ${end.toLocaleTimeString(locale, { hour: "2-digit", minute: "2-digit" })}`
          : ""}
      </p>

      {event.location ? (
        <p className="sf-muted" style={{ margin: "0.3rem 0 0" }}>
          <FiMapPin style={{ verticalAlign: "-2px" }} /> {event.location}
        </p>
      ) : null}

      {event.clubName ? (
        <p className="sf-muted" style={{ margin: "0.3rem 0 0" }}>
          {event.clubName}
        </p>
      ) : null}

      {event.description ? (
        <p style={{ whiteSpace: "pre-wrap", marginTop: "0.8rem" }}>
          <LinkifiedText text={event.description} />
        </p>
      ) : null}

      {event.url ? (
        <a
          href={event.url}
          target="_blank"
          rel="noopener noreferrer"
          className="sf-row"
          style={{ gap: "0.3rem", marginTop: "0.6rem" }}
        >
          <FiExternalLink /> {t("events.moreInfo")}
        </a>
      ) : null}

      {/* A birthday is generated from somebody's profile, so there is nothing
          here to edit or delete - it goes when they clear the date. */}
      {event.editable ? (
        <div className="sf-row" style={{ gap: "0.5rem", marginTop: "1rem" }}>
          <button className="sf-button sf-button-secondary sf-button-sm" onClick={onEdit}>
            <FiEdit2 /> {t("common.edit")}
          </button>
          <button className="sf-button sf-button-danger sf-button-sm" onClick={onDelete}>
            <FiTrash2 /> {t("common.delete")}
          </button>
        </div>
      ) : null}
    </Modal>
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
function EventFormModal({ event, clubs, canPostGlobally, canPublish, onClose, onSaved }) {
  const { t } = useI18n();
  const [form, setForm] = useState(() => ({
    clubId: event.clubId || (clubs.length === 1 && !canPostGlobally ? clubs[0].id : ""),
    title: event.title || "",
    description: event.description || "",
    location: event.location || "",
    url: event.url || "",
    kind: event.kind || "competition",
    color: event.color || randomColor(),
    // startsOn arrives when the form was opened by clicking a day in the grid:
    // that day at 09:00, which is a working guess somebody adjusts rather than
    // a blank field they have to fill from nothing.
    allDay: Boolean(event.allDay),
    startsAt: toInputValue(event.startsAt) || defaultStart(event.startsOn, event.fromMinutes),
    endsAt: toInputValue(event.endsAt) || defaultStart(event.startsOn, event.toMinutes),
    // A member with nowhere to publish is writing in their own calendar, so
    // that is what the form starts on rather than an option they cannot use.
    visibility: event.visibility || (canPublish ? "public" : "private"),
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
      // A private event is nobody's club's: sending one would put it on a
      // calendar its author did not choose.
      const payload = {
        ...form,
        clubId: form.visibility === "private" ? "" : form.clubId,
        // An all-day event is sent as midnight to midnight in the author's own
        // zone - the same convention as every other event here, where a time is
        // a wall-clock moment where the thing happens. The end is the *start*
        // of the day after the last one, which is what iCalendar means by an
        // exclusive DTEND and what the week grid's all-day row reads.
        startsAt: toInstant(form.allDay ? startOfDay(form.startsAt) : form.startsAt),
        endsAt: form.allDay
          ? toInstant(nextDay(form.endsAt || form.startsAt))
          : form.endsAt
            ? toInstant(form.endsAt)
            : "",
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
          {!event.id && canPublish && form.visibility !== "private" ? (
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

        {/* All day, the way cwclock's calendar does it: a switch that turns
            both pickers from datetime-local into plain dates. Unlike cwclock's
            time records - which carry a single day - an event has its own end,
            so a multi-day camp stays editable rather than being pinned to one
            day. */}
        <div className="sf-switch-field">
          <Switch
            id="allDay"
            checked={form.allDay}
            onChange={(fieldEvent) => {
              const allDay = fieldEvent.target.checked;
              setForm((current) => ({
                ...current,
                allDay,
                // Keep whatever day was already picked, dropping the time when
                // switching on: a date input rejects a value carrying one.
                startsAt: allDay ? startOfDay(current.startsAt) : current.startsAt,
                endsAt: allDay ? startOfDay(current.endsAt) : current.endsAt,
              }));
            }}
            aria-label={t("events.allDay")}
          />
          <label className="sf-label" htmlFor="allDay" style={{ margin: 0 }}>
            {t("events.allDay")}
          </label>
        </div>

        <div className="sf-row" style={{ gap: "0.6rem" }}>
          <div className="sf-field" style={{ flex: 1, minWidth: 170 }}>
            <label className="sf-label" htmlFor="startsAt">
              {t("events.startsAt")}
            </label>
            <input
              id="startsAt"
              className="sf-input"
              type={form.allDay ? "date" : "datetime-local"}
              value={form.allDay ? dayPart(form.startsAt) : form.startsAt}
              onChange={set("startsAt")}
              required
            />
          </div>
          <div className="sf-field" style={{ flex: 1, minWidth: 170 }}>
            <label className="sf-label" htmlFor="endsAt">
              {t(form.allDay ? "events.lastDay" : "events.endsAt")}{" "}
              <span className="sf-muted">({t("common.optional")})</span>
            </label>
            <input
              id="endsAt"
              className="sf-input"
              type={form.allDay ? "date" : "datetime-local"}
              value={form.allDay ? dayPart(form.endsAt) : form.endsAt}
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
          <label className="sf-label" htmlFor="color">
            {t("events.color")}
          </label>
          <div className="sf-color-field">
            <input
              id="color"
              className="sf-color-input"
              type="color"
              value={/^#[0-9a-f]{6}$/i.test(form.color) ? form.color : "#0e5e9b"}
              onChange={set("color")}
            />
            {/* The swatches are the quick path; the picker beside them is for
                anybody who wants a specific colour. */}
            <div className="sf-color-swatches">
              {EVENT_COLORS.map((color) => (
                <button
                  key={color}
                  type="button"
                  className={`sf-color-swatch ${form.color === color ? "active" : ""}`}
                  style={{ backgroundColor: color }}
                  onClick={() => setForm((current) => ({ ...current, color }))}
                  aria-label={color}
                />
              ))}
            </div>
          </div>
        </div>

        <div className="sf-field">
          <label className="sf-label" htmlFor="visibility">
            {t("events.visibility")}
          </label>
          <select id="visibility" className="sf-select" value={form.visibility} onChange={set("visibility")}>
            {/* Only somebody with a club to publish to is offered the wider
                audiences: an option that always fails is worse than its
                absence. */}
            {canPublish ? <option value="public">{t("events.visibilityPublic")}</option> : null}
            {canPublish ? <option value="club">{t("events.visibilityClub")}</option> : null}
            <option value="private">{t("events.visibilityPrivate")}</option>
          </select>
          <p className="sf-muted" style={{ fontSize: "0.82rem", marginBottom: 0 }}>
            {form.visibility === "private" ? t("events.visibilityPrivateHelp") : null}
          </p>
        </div>
      </form>
    </Modal>
  );
}

/** Subscribe-from-Outlook/Google panel: the feed URL, and how to revoke it. */
/**
 * Uploads a federation season calendar.
 *
 * The FFForce publishes its season as a PDF year planner. Retyping forty
 * competitions out of it is the kind of work nobody does twice, so the file
 * goes straight to the API, which reads the dates and the category colours off
 * the page. Re-uploading a revised calendar adds what changed rather than a
 * second copy of the year, which is what the "already there" count reports.
 */
function CalendarImport({ clubs, canPostGlobally, onImported }) {
  const { t } = useI18n();
  const input = useRef(null);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [clubId, setClubId] = useState(clubs.length === 1 && !canPostGlobally ? clubs[0].id : "");
  const [result, setResult] = useState(null);

  const upload = async (file) => {
    if (!file) return;
    setBusy(true);
    try {
      const imported = await eventsApi.importCalendar(file, clubId, clubId ? "club" : "public");
      setResult(imported);
      toast.success(t("events.imported", { count: imported.events.length }), toastOptions);
      await onImported();
    } catch (err) {
      toast.error(err?.response?.data?.message || t("errors.generic"), toastOptions);
    } finally {
      setBusy(false);
      // Cleared so the same file can be picked again after a failure - a file
      // input fires nothing when re-selecting the value it already holds.
      if (input.current) input.current.value = "";
    }
  };

  return (
    <>
      <button className="sf-button sf-button-secondary" onClick={() => setOpen(true)}>
        <FiUpload /> {t("events.import")}
      </button>

      {open ? (
        <Modal title={t("events.import")} onClose={() => setOpen(false)}>
          <p className="sf-muted" style={{ marginTop: 0 }}>
            {t("events.importHelp")}
          </p>

          {clubs.length > 0 ? (
            <div className="sf-field">
              <label className="sf-label" htmlFor="importClub">
                {t("events.club")}
              </label>
              <select id="importClub" className="sf-select" value={clubId} onChange={(e) => setClubId(e.target.value)}>
                {canPostGlobally ? <option value="">{t("events.noClub")}</option> : null}
                {clubs.map((club) => (
                  <option key={club.id} value={club.id}>
                    {club.name}
                  </option>
                ))}
              </select>
            </div>
          ) : null}

          <input
            ref={input}
            type="file"
            accept="application/pdf,.pdf"
            className="sf-input"
            disabled={busy}
            onChange={(event) => upload(event.target.files?.[0])}
          />

          {busy ? <p className="sf-muted">{t("events.importing")}</p> : null}

          {result ? (
            <div style={{ marginTop: "0.8rem" }}>
              <p style={{ margin: 0 }}>
                <strong>{t("events.imported", { count: result.events.length })}</strong>
              </p>
              {result.skipped > 0 ? (
                <p className="sf-muted" style={{ margin: "0.2rem 0 0" }}>
                  {t("events.importSkipped", { count: result.skipped })}
                </p>
              ) : null}
              {result.warnings?.length ? (
                <>
                  {/* Named rather than counted: these are the entries a coach
                      has to add by hand, and they need to know which. */}
                  <p className="sf-muted" style={{ margin: "0.4rem 0 0.2rem" }}>{t("events.importWarnings")}</p>
                  <ul className="sf-muted" style={{ margin: 0, paddingLeft: "1.1rem" }}>
                    {result.warnings.map((warning, index) => (
                      <li key={`${warning}-${index}`}>{warning}</li>
                    ))}
                  </ul>
                </>
              ) : null}
            </div>
          ) : null}
        </Modal>
      ) : null}
    </>
  );
}

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
function toInputValue(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (n) => String(n).padStart(2, "0");
  const day = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  return `${day}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/**
 * The datetime-local value for a day picked in the grid.
 *
 * With minutes given - the week view's drag - it is exactly the time that was
 * selected. Without them, the month view picked a day but no hour, so it
 * defaults to nine in the morning: a working guess to adjust rather than an
 * empty field to fill from nothing.
 */
function defaultStart(day, minutes) {
  if (!day) return "";
  const at = new Date(day);
  if (Number.isNaN(at.getTime())) return "";
  if (minutes == null) {
    at.setHours(9, 0, 0, 0);
  } else {
    // A selection running to the end of the day lands on the next midnight,
    // which is the correct instant for an event that ends at 24:00.
    at.setHours(0, minutes, 0, 0);
  }
  return toInputValue(at.toISOString());
}

/** The date half of an input value: what a `date` input accepts. */
function dayPart(value) {
  return (value || "").slice(0, 10);
}

/** Midnight on the day of an input value, as an input value. */
function startOfDay(value) {
  const day = dayPart(value);
  return day ? `${day}T00:00` : "";
}

/**
 * Midnight on the day *after* an input value.
 *
 * An all-day event's end is exclusive - iCalendar's DTEND, and what the grids
 * read - so an event whose last day is the 9th ends at the start of the 10th.
 * The author picks the last day, which is what they mean by it.
 */
function nextDay(value) {
  const day = dayPart(value);
  if (!day) return "";
  const at = new Date(`${day}T00:00`);
  if (Number.isNaN(at.getTime())) return "";
  at.setDate(at.getDate() + 1);
  return toInputValue(at.toISOString());
}

/** The inverse: a local input value back to an RFC 3339 instant. */
function toInstant(value) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}
