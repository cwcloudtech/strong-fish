import { useTheme } from "../../context/ThemeContext";

/**
 * The wordmark, inked for the current theme. The stock logo is drawn in navy
 * for a light background and would all but disappear on a dark one, so dark
 * mode gets the light-inked variant instead (see the asset generation in the
 * repo README).
 *
 * `mark` switches to the square fish-and-barbell crop, which is what's legible
 * at small sizes - the wordmark isn't.
 */
export default function Logo({ mark = false, on, className, style, alt = "strong-fish" }) {
  const { theme } = useTheme();
  // `on` pins the variant for a surface whose colour doesn't follow the theme -
  // the sidebar is navy in both themes, so its mark is always the light-inked
  // one regardless of what the rest of the app is doing.
  const resolved = on || theme;
  const suffix = resolved === "dark" ? "-dark" : "";
  const src = mark ? `/logo192${suffix}.png` : `/logo512${suffix}.png`;

  return <img className={className} style={style} src={src} alt={alt} />;
}
