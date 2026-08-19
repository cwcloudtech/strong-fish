import { useMemo } from "react";
import { FiChevronLeft, FiChevronRight } from "react-icons/fi";

import WeekGrid from "./WeekGrid";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The month and week grid, the way Outlook's meeting calendar works: the dates
 * laid out as a grid, each event a chip in its day, click one to open it and
 * click empty space to add one there.
 *
 * A list of dates answers "what is next"; a grid answers "how busy is that
 * week", which is the question somebody planning a training block actually
 * has. Both are offered - the list is still the better read on a phone.
 *
 * Ported from ~/cwclock's Calendar, adapted to events that are instants rather
 * than a day plus a time of day.
 */

const startOfDay = (date) => {
  const copy = new Date(date);
  copy.setHours(0, 0, 0, 0);
  return copy;
};

const addDays = (date, days) => {
  const copy = new Date(date);
  copy.setDate(copy.getDate() + days);
  return copy;
};

/**
 * Monday-first, which is what a European lifter expects and what every meet
 * schedule is written against.
 */
const startOfWeek = (date) => {
  const weekday = date.getDay();
  return addDays(startOfDay(date), weekday === 0 ? -6 : 1 - weekday);
};

/**
 * Moves by whole months, clamping the day to the target month's last.
 *
 * Native Date overflows instead: the 31st of January minus one month lands in
 * March, so paging back through a calendar would silently skip February.
 */
const shiftMonth = (date, delta) => {
  const target = new Date(date.getFullYear(), date.getMonth() + delta, 1);
  const lastDay = new Date(target.getFullYear(), target.getMonth() + 1, 0).getDate();
  target.setDate(Math.min(date.getDate(), lastDay));
  return target;
};

const isoDay = (date) =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;

// Beyond this a cell would grow and break the grid's even rows, so the rest
// collapse into a "+N" the day click opens.
const MAX_CHIPS = 3;

export default function EventCalendar({
  events,
  anchor,
  view,
  onAnchorChange,
  onViewChange,
  onSelect,
  onAddOn,
  onAddRange,
  canCreate,
}) {
  const { t, locale } = useI18n();
  const intlLocale = locale === "fr" ? "fr-FR" : "en-US";

  const monthStart = useMemo(
    () => new Date(anchor.getFullYear(), anchor.getMonth(), 1),
    [anchor]
  );
  const monthEnd = useMemo(
    () => new Date(anchor.getFullYear(), anchor.getMonth() + 1, 0),
    [anchor]
  );

  // Both views are the same grid: a week is one row of it, a month is every
  // row overlapping the month.
  const gridStart = useMemo(
    () => (view === "week" ? startOfWeek(anchor) : startOfWeek(monthStart)),
    [view, anchor, monthStart]
  );
  const gridEnd = useMemo(
    () => (view === "week" ? addDays(gridStart, 6) : addDays(startOfWeek(monthEnd), 6)),
    [view, gridStart, monthEnd]
  );

  const weeks = useMemo(() => {
    const days = [];
    for (let day = new Date(gridStart); day <= gridEnd; day = addDays(day, 1)) {
      days.push(day);
    }
    const rows = [];
    for (let i = 0; i < days.length; i += 7) rows.push(days.slice(i, i + 7));
    return rows;
  }, [gridStart, gridEnd]);

  /**
   * Events bucketed by the days they cover, so a camp running Friday to Sunday
   * appears on all three rather than only on the day it began.
   */
  const byDay = useMemo(() => {
    const map = {};
    for (const event of events) {
      const start = new Date(event.startsAt);
      if (Number.isNaN(start.getTime())) continue;

      const end = event.endsAt ? new Date(event.endsAt) : start;
      const last = Number.isNaN(end.getTime()) ? start : end;

      for (let day = startOfDay(start); day <= last; day = addDays(day, 1)) {
        (map[isoDay(day)] ||= []).push(event);
        // A runaway end date must not spin here.
        if (day > addDays(startOfDay(start), 366)) break;
      }
    }
    for (const key of Object.keys(map)) {
      map[key].sort((a, b) => new Date(a.startsAt) - new Date(b.startsAt));
    }
    return map;
  }, [events]);

  const todayIso = isoDay(new Date());

  const weekdayLabels = useMemo(() => {
    // Any known Monday works as the seed for the localized short names.
    const monday = startOfWeek(new Date());
    return Array.from({ length: 7 }, (_, i) =>
      new Intl.DateTimeFormat(intlLocale, { weekday: "short" }).format(addDays(monday, i))
    );
  }, [intlLocale]);

  const rangeLabel = useMemo(() => {
    if (view === "week") {
      const day = new Intl.DateTimeFormat(intlLocale, { day: "numeric", month: "short" });
      const year = new Intl.DateTimeFormat(intlLocale, { year: "numeric" });
      return `${day.format(gridStart)} – ${day.format(gridEnd)} ${year.format(gridEnd)}`;
    }
    return new Intl.DateTimeFormat(intlLocale, { month: "long", year: "numeric" }).format(monthStart);
  }, [view, gridStart, gridEnd, monthStart, intlLocale]);

  const step = (delta) =>
    onAnchorChange(view === "week" ? addDays(anchor, delta * 7) : shiftMonth(anchor, delta));

  return (
    <div className="sf-calendar">
      <div className="sf-calendar-header">
        <h2 className="sf-calendar-range">{rangeLabel}</h2>

        <div className="sf-calendar-nav">
          <div className="sf-calendar-views">
            <button
              type="button"
              className={`sf-calendar-view ${view === "month" ? "active" : ""}`}
              onClick={() => onViewChange("month")}
            >
              {t("events.monthView")}
            </button>
            <button
              type="button"
              className={`sf-calendar-view ${view === "week" ? "active" : ""}`}
              onClick={() => onViewChange("week")}
            >
              {t("events.weekView")}
            </button>
          </div>

          <button
            type="button"
            className="sf-button sf-button-secondary sf-button-sm"
            onClick={() => onAnchorChange(startOfDay(new Date()))}
          >
            {t("events.today")}
          </button>
          <button
            type="button"
            className="sf-calendar-step"
            onClick={() => step(-1)}
            aria-label={t("events.previous")}
          >
            <FiChevronLeft />
          </button>
          <button
            type="button"
            className="sf-calendar-step"
            onClick={() => step(1)}
            aria-label={t("events.next")}
          >
            <FiChevronRight />
          </button>
        </div>
      </div>

      {view === "week" ? (
        <WeekGrid
          days={(weeks[0] || []).map((date, index) => ({
            date,
            iso: isoDay(date),
            label: `${weekdayLabels[index]} `,
            weekend: date.getDay() === 0 || date.getDay() === 6,
          }))}
          byDay={byDay}
          todayIso={todayIso}
          onSelectEvent={onSelect}
          onSelectRange={onAddRange}
          canCreate={canCreate}
        />
      ) : (
      <>
      <div className="sf-calendar-weekdays">
        {weekdayLabels.map((label) => (
          <div key={label} className="sf-calendar-weekday">
            {label}
          </div>
        ))}
      </div>

      <div className="sf-calendar-grid">
        {weeks.map((week, index) => (
          <div key={index} className="sf-calendar-week">
            {week.map((date) => {
              const iso = isoDay(date);
              const dayEvents = byDay[iso] || [];
              const visible = dayEvents.slice(0, MAX_CHIPS);
              const hidden = dayEvents.length - visible.length;

              return (
                <div
                  key={iso}
                  className={[
                    "sf-calendar-day",
                    view === "month" && date.getMonth() !== monthStart.getMonth() ? "outside" : "",
                    iso === todayIso ? "today" : "",
                    date.getDay() === 0 || date.getDay() === 6 ? "weekend" : "",
                    canCreate ? "creatable" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                  // Clicking the empty part of a day starts an event there,
                  // which is the gesture people already have from Outlook.
                  onClick={canCreate ? () => onAddOn(date) : undefined}
                >
                  <span className="sf-calendar-date">{date.getDate()}</span>

                  <div className="sf-calendar-chips">
                    {visible.map((event) => (
                      <button
                        key={`${event.id}-${iso}`}
                        type="button"
                        className={`sf-calendar-chip sf-calendar-chip-${event.kind}`}
                        title={event.title}
                        onClick={(clickEvent) => {
                          // Otherwise the day underneath also fires and opens
                          // an empty form on top of the event.
                          clickEvent.stopPropagation();
                          onSelect(event);
                        }}
                      >
                        <span className="sf-calendar-chip-time">{chipTime(event, intlLocale)}</span>
                        <span className="sf-calendar-chip-title">{event.title}</span>
                      </button>
                    ))}
                    {hidden > 0 ? (
                      <span className="sf-calendar-more">{t("events.more", { count: hidden })}</span>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        ))}
      </div>
      </>
      )}
    </div>
  );
}

/**
 * The chip's leading time. Birthdays occupy the whole day and have no hour
 * worth showing; everything else starts when it starts.
 */
function chipTime(event, intlLocale) {
  if (event.kind === "birthday") return "";
  const start = new Date(event.startsAt);
  if (Number.isNaN(start.getTime())) return "";
  return new Intl.DateTimeFormat(intlLocale, { hour: "2-digit", minute: "2-digit" }).format(start);
}
