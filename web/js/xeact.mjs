/**
 * Generate a relative URL from `url`, appending all key-value pairs from `params` as URL-encoded parameters.
 *
 * @type{function(string=, Object=): string}
 */
export const u = (url = "", params = {}) => {
  let result = new URL(url, window.location.href);
  Object.entries(params).forEach((kv) => {
    let [k, v] = kv;
    result.searchParams.set(k, v);
  });
  return result.toString();
};