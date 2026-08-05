---
title: List of known browser extensions that can break Anubis
---

This page outlines browser extensions known to break Anubis' client code and workarounds.

## [JShelter](https://jshelter.org/)

| Extension    | JShelter                                                                                                                                           |
| :----------- | :------------------------------------------------------------------------------------------------------------------------------------------------- |
| Website      | [jshelter.org](https://jshelter.org/)                                                                                                              |
| GitHub issue | https://github.com/TecharoHQ/anubis/issues/25                                                                                                      |
| Be aware of  | [What are Web Workers, and what are the threats that I face?](https://jshelter.org/faq/#what-are-web-workers-and-what-are-the-threats-that-i-face) |

### Workaround 1

Disable WebWorker protection for the given site.

1. Click on the JShelter badge icon.
2. Expand JavaScript Shield settings by clicking on the `Modify` button.
3. Click on the `Detail tweaks of JS shield for this site` button.
4. Click and drag the `WebWorker` slider to the left, showing `Unprotected`.
5. Refresh the page.

### Workaround 2

1. Click on the JShelter badge icon.
2. Expand JavaScript Shield settings by clicking on the `Modify` button.
3. Choose "Turn JavaScript Shield off"
4. Refresh the page.

:::note

Taking these actions will remove all protections of JavaScript Shield for all pages at the visited web site. You might want review and amend your JavaScript shield settings once you go through the challenge based on your operational security model.

:::

### Workaround 3

1. Open JShelter extension settings
2. Click on JS Shield details
3. Enter in the domain for a website protected by Anubis
4. Choose "Turn JavaScript Shield off"
5. Hit "Add to list"

:::note

Taking these actions will remove all protections of JavaScript Shield for all pages at the visited web site. You might want review and amend your JavaScript shield settings once you go through the challenge based on your operational security model.

:::
