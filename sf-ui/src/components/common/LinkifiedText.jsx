import React from "react";

/**
 * Any http(s) run up to the next whitespace. Deliberately greedy: a URL with
 * a query string, a fragment or parentheses in it is still one URL, and
 * whitespace is the only delimiter a person typing into a text box gives us.
 */
const URL_PATTERN = /(https?:\/\/[^\s]+)/g;

/** Punctuation that ends a sentence rather than a URL. */
const TRAILING = `.,;:!?)]}"'`;

/**
 * Splits a matched run into the URL itself and whatever punctuation trailed
 * it, so "see https://example.com." doesn't link to a URL ending in a period.
 */
function splitTrailing(url) {
  let end = url.length;
  while (end > 0 && TRAILING.includes(url[end - 1])) {
    end -= 1;
  }
  return [url.slice(0, end), url.slice(end)];
}

/**
 * Renders text with any http(s) URL turned into a clickable link.
 *
 * Built as React nodes rather than with dangerouslySetInnerHTML: this renders
 * text somebody else wrote - posts, comments, private messages - and building
 * markup from it would be an XSS vector. React escapes every part it renders,
 * and only the href of a run that already matched http(s) is ever a link.
 *
 * `rel="noopener noreferrer"` because the target is a stranger's URL: without
 * it the opened page gets a handle on this one through window.opener.
 */
export default function LinkifiedText({ text, className }) {
  const content = String(text ?? "");

  return (
    <>
      {content.split(URL_PATTERN).map((part, index) => {
        if (!/^https?:\/\//.test(part)) {
          return <React.Fragment key={index}>{part}</React.Fragment>;
        }
        const [href, trailing] = splitTrailing(part);
        return (
          <React.Fragment key={index}>
            <a href={href} target="_blank" rel="noopener noreferrer" className={className || "sf-inline-link"}>
              {href}
            </a>
            {trailing}
          </React.Fragment>
        );
      })}
    </>
  );
}
