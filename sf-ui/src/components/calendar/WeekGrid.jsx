import { useEffect, useRef, useState } from "react";

import { useI18n } from "../../i18n/I18nContext";
import isWholeDay from "../../utils/wholeDay";

/**
 * The week as an hour-by-hour grid, where dragging down a column selects a time
 * range and opens the form prefilled with it.
 *
 * A month cell can only say "something happens that day". A coach scheduling a
 * session is choosing an hour, and the natural gesture for that - the one
 * Outlook and Google Calendar both use - is to drag across the time you want.
 * Ported from ~/cwclock's CalendarWeekView.
 */

const HOUR_HEIGHT = 44;

/**
 * Each column is 48 half-hour slots rather than 24 hourly ones, so a selection
 * can start and end on the half hour - which is where sessions actually start.
 */
const SLOTS_PER_DAY = 48;
const SLOT_HEIGHT = HOUR_HEIGHT / 2;
const HOURS = Array.from({ length: 24 }, (_, hour) => hour);
const SLOTS = Array.from({ length: SLOTS_PER_DAY }, (_, slot) => slot);

/** Opening at midnight would put the useful part of the day off-screen. */
const INITIAL_HOUR = 7;

/** A slot index as minutes past midnight. */
const slotMinutes = (slot) => slot * 30;

export default function WeekGrid({ days, byDay, todayIso, onSelectRange, onSelectEvent, canCreate }) {
  const { t, locale } = useI18n();
  const intlLocale = locale === "fr" ? "fr-FR" : "en-US";
  const scrollRef = useRef(null);
  // { iso, date, startSlot, endSlot } while the pointer is down.
  const [drag, setDrag] = useState(null);

  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = INITIAL_HOUR * HOUR_HEIGHT;
  }, []);

  /**
   * The gesture ends wherever the pointer is released - including outside the
   * grid, which is why this listens on the window rather than on the cells.
   *
   * A plain click leaves start and end on the same slot, and becomes a single
   * half-hour block: one handler covers both "click a slot" and "drag across
   * several".
   */
  useEffect(() => {
    if (!drag) return undefined;

    const finish = () => {
      setDrag((current) => {
        if (current) {
          const from = Math.min(current.startSlot, current.endSlot);
          const to = Math.max(current.startSlot, current.endSlot) + 1;
          onSelectRange(current.date, slotMinutes(from), slotMinutes(to));
        }
        return null;
      });
    };

    window.addEventListener("mouseup", finish);
    return () => window.removeEventListener("mouseup", finish);
    // onSelectRange is stable enough in practice; re-subscribing on every
    // render would tear the listener down mid-gesture.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [drag]);

  const selected = (iso, slot) => {
    if (!drag || drag.iso !== iso) return false;
    const low = Math.min(drag.startSlot, drag.endSlot);
    const high = Math.max(drag.startSlot, drag.endSlot);
    return slot >= low && slot <= high;
  };

  return (
    <div className="sf-week">
      <div className="sf-week-head">
        <div className="sf-week-axis-head" />
        {days.map((day) => (
          <div
            key={day.iso}
            className={`sf-week-day-head ${day.iso === todayIso ? "today" : ""} ${day.weekend ? "weekend" : ""}`}
          >
            <span className="sf-week-day-name">{day.label}</span>
            <span className="sf-week-day-number">{day.date.getDate()}</span>
          </div>
        ))}
      </div>

      {/* All-day entries get their own row above the hours, the way cwclock's
          week view does it - and Outlook, and Google. They have no hour to be
          placed at, and stretching one down a column would claim a time its
          author never gave. */}
      <div className="sf-week-allday">
        <div className="sf-week-allday-label">{t("events.allDay")}</div>
        {days.map((day) => (
          <div key={day.iso} className={`sf-week-allday-cell ${day.weekend ? "weekend" : ""}`}>
            {(byDay[day.iso] || []).filter(isWholeDay).map((event) => (
              <button
                key={`${event.id}-${day.iso}-allday`}
                type="button"
                className={`sf-calendar-chip sf-calendar-chip-${event.kind}`}
                style={event.color ? { color: event.color } : undefined}
                title={event.title}
                onClick={() => onSelectEvent(event)}
              >
                <span className="sf-calendar-chip-title">{event.title}</span>
              </button>
            ))}
          </div>
        ))}
      </div>

      <div className="sf-week-body" ref={scrollRef}>
        <div className="sf-week-axis">
          {HOURS.map((hour) => (
            <div key={hour} className="sf-week-hour-label" style={{ height: HOUR_HEIGHT }}>
              {/* Rendered from a real date so 13:00 or 1 PM follows the locale
                  rather than being hard-coded to one of them. */}
              {new Intl.DateTimeFormat(intlLocale, { hour: "2-digit", minute: "2-digit" }).format(
                new Date(2020, 0, 1, hour, 0)
              )}
            </div>
          ))}
        </div>

        {days.map((day) => (
          <div key={day.iso} className={`sf-week-column ${day.weekend ? "weekend" : ""}`}>
            {SLOTS.map((slot) => (
              <div
                key={slot}
                className={[
                  "sf-week-slot",
                  slot % 2 === 1 ? "half" : "",
                  selected(day.iso, slot) ? "selected" : "",
                  canCreate ? "creatable" : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                style={{ height: SLOT_HEIGHT }}
                onMouseDown={
                  canCreate
                    ? (event) => {
                        // Left button only: a right-click is a context menu,
                        // and a middle-click is a scroll.
                        if (event.button !== 0) return;
                        event.preventDefault();
                        setDrag({ iso: day.iso, date: day.date, startSlot: slot, endSlot: slot });
                      }
                    : undefined
                }
                onMouseEnter={
                  canCreate
                    ? () =>
                        setDrag((current) =>
                          // Only extends within the column it started in: a
                          // range spanning two days is not something the form
                          // can express.
                          current && current.iso === day.iso ? { ...current, endSlot: slot } : current
                        )
                    : undefined
                }
              />
            ))}

            {/* Whole-day entries live in the row above; drawing them here too
                would show each one twice. */}
            {(byDay[day.iso] || []).filter((event) => !isWholeDay(event)).map((event) => {
              const geometry = blockGeometry(event, day.date);
              if (!geometry) return null;
              return (
                <button
                  key={`${event.id}-${day.iso}`}
                  type="button"
                  className={`sf-week-event sf-calendar-chip-${event.kind}`}
                  style={{
                    top: geometry.top,
                    height: geometry.height,
                    // See EventCalendar: the block is drawn from currentColor.
                    ...(event.color ? { color: event.color } : {}),
                  }}
                  title={event.title}
                  onClick={(clickEvent) => {
                    clickEvent.stopPropagation();
                    onSelectEvent(event);
                  }}
                  // The event sits on top of the slots; without this a click on
                  // it would also start a selection underneath.
                  onMouseDown={(clickEvent) => clickEvent.stopPropagation()}
                >
                  <span className="sf-week-event-title">{event.title}</span>
                </button>
              );
            })}
          </div>
        ))}
      </div>

      {canCreate ? <p className="sf-muted sf-week-hint">{t("events.dragHint")}</p> : null}
    </div>
  );
}

/**
 * Where an event's block sits in its column, in pixels.
 *
 * Clamped to the day being drawn, so an event running past midnight fills the
 * rest of this column rather than overflowing it, and picks up again in the
 * next - which is what a reader expects a multi-day block to look like.
 */
function blockGeometry(event, day) {
  const start = new Date(event.startsAt);
  if (Number.isNaN(start.getTime())) return null;

  const end = event.endsAt ? new Date(event.endsAt) : new Date(start.getTime() + 60 * 60 * 1000);
  const dayStart = new Date(day);
  dayStart.setHours(0, 0, 0, 0);
  const dayEnd = new Date(dayStart.getTime() + 24 * 60 * 60 * 1000);

  const from = Math.max(start.getTime(), dayStart.getTime());
  const to = Math.min(Number.isNaN(end.getTime()) ? from : end.getTime(), dayEnd.getTime());
  if (to <= from) return null;

  const minutesFromMidnight = (from - dayStart.getTime()) / 60000;
  const minutes = (to - from) / 60000;

  return {
    top: (minutesFromMidnight / 60) * HOUR_HEIGHT,
    // A 30-minute session still has to be readable, so blocks have a floor.
    height: Math.max((minutes / 60) * HOUR_HEIGHT, 18),
  };
}
