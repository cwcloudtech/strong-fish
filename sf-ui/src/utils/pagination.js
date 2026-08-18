/**
 * How many rows a paged screen asks for at a time.
 *
 * One constant rather than a number repeated per screen: the page size is a
 * property of the app, and having each list pick its own is how a "load more"
 * ends up fetching 20 in one place and 25 in another.
 *
 * Overridable per deployment, since what reads well depends on the screens
 * people actually use it on.
 */
export const SF_PAGINATION_SIZE = Number(process.env.REACT_APP_PAGE_SIZE) || 20;

export default SF_PAGINATION_SIZE;
