// <media-player url="..."></media-player> - a plain native custom element (not a
// React component), so it can be reused outside this app too. Given a raw URL it
// detects the provider (YouTube/Vimeo/Dailymotion/Facebook video, SlideShare, or
// a direct image link) and renders the right embed, falling back to a link-out
// card for anything else.
//
// Ported from ~/uprodit/prodit-ui's webcomponents/MediaPlayer.ts: a post here
// carries raw links (see the API's Post.links), and this is what turns a pasted
// YouTube URL into an actual player.

const IMAGE_EXTENSION_PATTERN = /\.(png|jpe?g|gif|webp|svg|bmp|avif)(\?.*)?$/i;

function firstPathSegment(url, fromEnd = true) {
  const parts = url.pathname.split("/").filter(Boolean);
  if (parts.length === 0) return undefined;
  return fromEnd ? parts[parts.length - 1] : parts[0];
}

function hostnameWithoutWww(url) {
  return url.hostname.replace(/^www\./, "");
}

export function detectMedia(rawUrl) {
  let url;
  try {
    url = new URL(rawUrl);
  } catch {
    return { kind: "link", embedUrl: rawUrl };
  }

  const host = hostnameWithoutWww(url);

  if (host === "youtube.com" || host === "m.youtube.com") {
    const id = url.searchParams.get("v") || firstPathSegment(url);
    if (id) return { kind: "youtube", embedUrl: `https://www.youtube.com/embed/${id}` };
  }
  if (host === "youtu.be") {
    const id = firstPathSegment(url);
    if (id) return { kind: "youtube", embedUrl: `https://www.youtube.com/embed/${id}` };
  }

  if (host === "vimeo.com" || host === "player.vimeo.com") {
    const id = firstPathSegment(url);
    if (id && /^\d+$/.test(id)) return { kind: "vimeo", embedUrl: `https://player.vimeo.com/video/${id}` };
  }

  if (host === "dailymotion.com") {
    const parts = url.pathname.split("/").filter(Boolean);
    const videoIndex = parts.indexOf("video");
    const id = videoIndex >= 0 ? parts[videoIndex + 1] : parts[parts.length - 1];
    if (id) return { kind: "dailymotion", embedUrl: `https://www.dailymotion.com/embed/video/${id}` };
  }
  if (host === "dai.ly") {
    const id = firstPathSegment(url);
    if (id) return { kind: "dailymotion", embedUrl: `https://www.dailymotion.com/embed/video/${id}` };
  }

  if (host === "facebook.com" || host === "fb.watch") {
    return {
      kind: "facebook",
      embedUrl: `https://www.facebook.com/plugins/video.php?href=${encodeURIComponent(rawUrl)}&show_text=false`,
    };
  }

  if (host === "slideshare.net") {
    // SlideShare's real embed code needs a numeric id resolved through their
    // oEmbed API, which has no CORS support - so it can't be resolved
    // client-side. Rendered as a link-out card rather than a broken iframe.
    return { kind: "slideshare", embedUrl: rawUrl };
  }

  if (IMAGE_EXTENSION_PATTERN.test(url.pathname)) {
    return { kind: "image", embedUrl: rawUrl };
  }

  return { kind: "link", embedUrl: rawUrl };
}

const VIDEO_KINDS = ["youtube", "vimeo", "dailymotion", "facebook"];

const STYLE = `
  :host { display: block; }
  .frame {
    position: relative;
    padding-bottom: 56.25%; /* 16:9 */
    height: 0;
    overflow: hidden;
    border-radius: 10px;
    background-color: #0000000d;
  }
  .frame iframe { position: absolute; inset: 0; width: 100%; height: 100%; border: 0; }
  .image {
    display: block;
    width: 100%;
    max-height: 480px;
    object-fit: cover;
    border-radius: 10px;
  }
  .link-card {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.85rem 1rem;
    border: 1px solid #dbe3ec;
    border-radius: 10px;
    background-color: #f8fafc;
    color: #062a4e;
    text-decoration: none;
    font: inherit;
    font-size: 0.85rem;
    word-break: break-word;
  }
  .link-card:hover { border-color: #0e5e9b; }
  .link-icon {
    flex-shrink: 0;
    width: 34px;
    height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background-color: rgba(14, 94, 155, 0.12);
    font-size: 0.95rem;
  }
`;

const LINK_ICON_BY_KIND = { slideshare: "📊", link: "🔗" };

class MediaPlayerElement extends HTMLElement {
  static get observedAttributes() {
    return ["url"];
  }

  connectedCallback() {
    this.render();
  }

  attributeChangedCallback() {
    this.render();
  }

  render() {
    const url = this.getAttribute("url");
    const root = this.shadowRoot ?? this.attachShadow({ mode: "open" });
    root.innerHTML = "";

    if (!url) return;

    const style = document.createElement("style");
    style.textContent = STYLE;
    root.appendChild(style);

    const media = detectMedia(url);

    if (VIDEO_KINDS.includes(media.kind)) {
      const frame = document.createElement("div");
      frame.className = "frame";
      const iframe = document.createElement("iframe");
      iframe.src = media.embedUrl;
      iframe.setAttribute(
        "allow",
        "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
      );
      iframe.allowFullscreen = true;
      frame.appendChild(iframe);
      root.appendChild(frame);
      return;
    }

    if (media.kind === "image") {
      const img = document.createElement("img");
      img.className = "image";
      img.src = media.embedUrl;
      img.loading = "lazy";
      img.alt = "";
      root.appendChild(img);
      return;
    }

    // slideshare / generic link: a link-out card rather than a live embed.
    const link = document.createElement("a");
    link.className = "link-card";
    link.href = url;
    link.target = "_blank";
    link.rel = "noopener noreferrer";

    const icon = document.createElement("span");
    icon.className = "link-icon";
    icon.textContent = LINK_ICON_BY_KIND[media.kind] || "🔗";

    const label = document.createElement("span");
    label.textContent = url;

    link.appendChild(icon);
    link.appendChild(label);
    root.appendChild(link);
  }
}

export function defineMediaPlayer() {
  if (typeof customElements !== "undefined" && !customElements.get("media-player")) {
    customElements.define("media-player", MediaPlayerElement);
  }
}

export default MediaPlayerElement;
