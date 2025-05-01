---
title: List of known browser extensions that can break Anubis
---

This page contains a list of all of the browser extensions that are known to break Anubis' functionality and their associated GitHub issues, along with instructions on how to work around the issue.

## [JShelter](https://jshelter.org/)

| Extension    | JShelter                                                                                                                                           |
| :----------- | :------------------------------------------------------------------------------------------------------------------------------------------------- |
| Website      | [jshelter.org](https://jshelter.org/)                                                                                                              |
| GitHub issue | https://github.com/TecharoHQ/anubis/issues/25                                                                                                      |
| Be aware of  | [What are Web Workers, and what are the threats that I face?](https://jshelter.org/faq/#what-are-web-workers-and-what-are-the-threats-that-i-face) |

### Workaround steps (recommended):

1. Click on the JShelter badge icon (typically in the toolbar next to your navigation bar; if you cannot locate the icon, see [this question](https://jshelter.org/faq/#can-i-see-a-jshelter-badge-icon-next-to-my-navigation-bar-i-want-to-interact-with-the-extension-easily-and-avoid-going-through-settings)).
2. Expand JavaScript Shield settings by clicking on the `Modify` button.
3. Click on the `Detail tweaks of JS shield for this site` button.
4. Click and drag the `WebWorker` slider to the left until `Remove` is replaced by the `Unprotected`.
5. Refresh the page, for example, by clicking on the `Refresh page` button at the top of the JShelter pop up window.
6. You might want to restore the Worker settings once you go through the challenge.

### Workaround steps (alternative if you do not want to dig in JShelter's pop up):

1. Click on the JShelter badge icon (typically in the toolbar next to your navigation bar; if you cannot locate the icon, see [this question](https://jshelter.org/faq/#can-i-see-a-jshelter-badge-icon-next-to-my-navigation-bar-i-want-to-interact-with-the-extension-easily-and-avoid-going-through-settings)).
2. Expand JavaScript Shield settings by clicking on the `Modify` button.
3. Choose "Turn JavaScript Shield off"
4. Refresh the page, for example, by clicking on the `Refresh page` button at the top of the JShelter pop up window.

:::note

Taking these actions will remove all protections of JavaScript Shield for all pages at the visited web site. You might want review and amend your JavaScript shield settings once you go through the challenge based on your operational security model.

:::

### Workaround steps (alternative if you do not like JShelter's pop up):

1. Open JShelter extension settings
2. Click on JS Shield details
3. Enter in the domain for a website protected by Anubis
4. Choose "Turn JavaScript Shield off"
5. Hit "Add to list"

:::note

Taking these actions will remove all protections of JavaScript Shield for all pages at the visited web site. You might want review and amend your JavaScript shield settings once you go through the challenge based on your operational security model.

:::
