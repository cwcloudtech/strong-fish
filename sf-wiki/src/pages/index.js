import { Redirect } from "@docusaurus/router";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";

/**
 * The site root sends readers to the documentation.
 *
 * There is no landing page: this is a wiki, and the docs are the product. The
 * root still has to resolve to something, though - the navbar logo points at
 * it, and without this every page would carry a broken link.
 */
export default function Home() {
  const { siteConfig } = useDocusaurusContext();
  return <Redirect to={`${siteConfig.baseUrl}docs`} />;
}
