const VideoEmbedUrl = (original_url) => {
  try {
    const url = new URL(original_url);

    if (url.hostname.includes("youtube.com")) {
      const v = url.searchParams.get("v");
      if (v) {
        return `https://www.youtube.com/embed/${v}`;
      }

      const parts = url.pathname.split("/");
      const last = parts[parts.length - 1];
      if (last) {
        return `https://www.youtube.com/embed/${last}`;
      }
    }

    if (url.hostname === "youtu.be") {
      const parts = url.pathname.split("/");
      const id = parts[parts.length - 1];
      if (id) {
        return `https://www.youtube.com/embed/${id}`;
      }
    }

    if (url.hostname.includes("vimeo.com")) {
      const parts = url.pathname.split("/").filter(Boolean);
      const last = parts[parts.length - 1];

      if (last && /^\d+$/.test(last)) {
        return `https://player.vimeo.com/video/${last}`;
      }

      return original_url;
    }

    if (
      url.hostname.includes("facebook.com") ||
      url.hostname.includes("fb.watch")
    ) {
      // Facebook utilise un embed spécial basé sur l'URL complète encodée
      return `https://www.facebook.com/plugins/video.php?href=${encodeURIComponent(
        original_url
      )}&show_text=false&height=314`;
    }

    return original_url;
  } catch {
    return original_url;
  }
}

export default VideoEmbedUrl;
