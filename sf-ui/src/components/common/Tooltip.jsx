import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

/**
 * A tooltip, replacing bare `title` attributes - whose native bubble is slow to
 * appear and can't be styled. Most noticeable on the collapsed sidebar, where
 * the icon is all there is to go on.
 *
 * The bubble is portaled to document.body and placed from the trigger's own
 * bounding rect, rather than being anchored in CSS next to it. That is not
 * decoration: the sidebar scrolls, and a scroll container clips its overflow in
 * *both* axes whatever `overflow-x` says - so a CSS-anchored bubble sitting to
 * the right of a nav link was clipped away entirely and never appeared. The
 * same reasoning is why ~/cwclock portals its calendar chips' tooltips.
 *
 * With no label it renders its child untouched, so a caller can switch it off
 * (`label={collapsed ? text : null}`) without branching on the whole element.
 */

/** Long enough not to fire while the pointer is only passing over. */
const OPEN_DELAY = 300;

/** The gap between the trigger and the bubble. */
const OFFSET = 10;

/** How close to the window's edge the bubble may sit before it is nudged in. */
const EDGE = 8;

export default function Tooltip({ label, position = "top", className = "", children }) {
  const wrapperRef = useRef(null);
  const bubbleRef = useRef(null);
  const timerRef = useRef(null);
  const [origin, setOrigin] = useState(null);
  const [shift, setShift] = useState(0);

  const hide = useCallback(() => {
    clearTimeout(timerRef.current);
    setOrigin(null);
  }, []);

  const show = useCallback(() => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      const rect = wrapperRef.current?.getBoundingClientRect();
      if (rect) setOrigin(rect);
    }, OPEN_DELAY);
  }, []);

  // Nudged back inside the window once its width is known.
  //
  // The bubble is centred on its trigger, and a trigger near the right edge -
  // an icon button in a heading's corner, which is most of them - pushed half
  // the bubble off screen, where it was simply cut off mid-word. Measured
  // rather than guessed, because the text is a translation and its width is
  // not knowable in advance. offsetWidth is used rather than the rendered
  // rect: the width does not depend on the nudge, so this cannot feed back on
  // itself.
  useLayoutEffect(() => {
    if (!origin) {
      setShift(0);
      return;
    }
    const width = bubbleRef.current?.offsetWidth;
    if (!width) return;

    const centred = position === "right" ? origin.right + OFFSET : origin.left + origin.width / 2 - width / 2;
    const clamped = Math.max(EDGE, Math.min(centred, window.innerWidth - EDGE - width));
    setShift(clamped - centred);
  }, [origin, position]);

  // A tooltip left behind by an unmounting trigger would hang on screen with
  // nothing to dismiss it.
  useEffect(() => () => clearTimeout(timerRef.current), []);

  // The bubble is positioned in viewport coordinates, so anything that moves
  // the trigger under it invalidates it. Closing is the honest response -
  // tracking a moving target would be a lot of work to keep a hint on screen.
  useEffect(() => {
    if (!origin) return undefined;
    window.addEventListener("scroll", hide, true);
    window.addEventListener("resize", hide);
    return () => {
      window.removeEventListener("scroll", hide, true);
      window.removeEventListener("resize", hide);
    };
  }, [origin, hide]);

  if (!label) return children;

  return (
    <span
      ref={wrapperRef}
      className={`sf-tooltip ${className}`}
      onMouseEnter={show}
      onMouseLeave={hide}
      // Keyboard users get the same hint. focus/blur rather than :focus-within,
      // which would also match the focus a mouse click leaves behind and pin
      // the bubble open after the pointer has gone.
      onFocus={show}
      onBlur={hide}
    >
      {children}
      {origin
        ? createPortal(
            <span
              ref={bubbleRef}
              className="sf-tooltip-bubble"
              data-position={position}
              role="tooltip"
              style={{ ...bubbleStyle(position, origin), marginLeft: shift }}
            >
              {label}
            </span>,
            document.body
          )
        : null}
    </span>
  );
}

/**
 * Where the bubble goes, in viewport coordinates. The transforms centre it on
 * the trigger; `position: fixed` in the stylesheet is what takes it out of the
 * scrolling container that would otherwise clip it.
 */
function bubbleStyle(position, rect) {
  switch (position) {
    case "right":
      return {
        top: rect.top + rect.height / 2,
        left: rect.right + OFFSET,
        transform: "translateY(-50%)",
      };
    case "bottom":
      return {
        top: rect.bottom + OFFSET,
        left: rect.left + rect.width / 2,
        transform: "translateX(-50%)",
      };
    default:
      return {
        top: rect.top - OFFSET,
        left: rect.left + rect.width / 2,
        transform: "translate(-50%, -100%)",
      };
  }
}
