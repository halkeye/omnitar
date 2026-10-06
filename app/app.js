import "./index-source.css";
import "./copy-paste-entry.js";

// Web Awesome styles
import "@awesome.me/webawesome/dist/styles/webawesome.css";
import "@awesome.me/webawesome/dist/styles/utilities.css";
import "@awesome.me/webawesome/dist/styles/native.css";

// Import the components you want to use
import "@awesome.me/webawesome/dist/components/page/page.js";
import "@awesome.me/webawesome/dist/components/button/button.js";
import "@awesome.me/webawesome/dist/components/toast/toast.js";
import "@awesome.me/webawesome/dist/components/toast-item/toast-item.js";
import "@awesome.me/webawesome/dist/components/callout/callout.js";
import "@awesome.me/webawesome/dist/components/icon/icon.js";
import "@awesome.me/webawesome/dist/components/tab/tab.js";
import "@awesome.me/webawesome/dist/components/tab-group/tab-group.js";
import "@awesome.me/webawesome/dist/components/tab-panel/tab-panel.js";

if (window.location.pathname === "/") {
  const ogFetch = window.fetch;
  window.fetch = async function fakeFetch(url) {
    const parsedUrl = new URL(url);
    if (
      parsedUrl.pathname.endsWith("/account/demo/profiles/auto/fake@fake.com")
    ) {
      return new Response("", { status: 404 });
    }
    if (
      parsedUrl.pathname.endsWith(
        "/account/demo/profiles/auto/slack@gavinmogan.com",
      )
    ) {
      return new Response(
        JSON.stringify({
          id: "U0AGP1U9L0G",
          team_id: "E0AFSMB25HU",
          email: "slack@gavinmogan.com",
          name: "Gavin Test User",
          avatar_url:
            "https://secure.gravatar.com/avatar/ba03004158a764a4a58c2c097fdb57d6.jpg?s=512\u0026d=https%3A%2F%2Fa.slack-edge.com%2Fdf10d%2Fimg%2Favatars%2Fava_0018-512.png",
          fields: {
            Country: "Canada",
            Manager: "Eliza Berry",
            github: "@halkeye",
          },
        }),
      );
    }
    if (parsedUrl.pathname.endsWith("/account/demo/issues/auto/TEST-404")) {
      return new Response("", { status: 404 });
    }
    if (parsedUrl.pathname.endsWith("/account/demo/issues/auto/TEST-200")) {
      return new Response(
        JSON.stringify({
          source: "jira",
          title: "adasd",
          url: "https://halkeye.atlassian.net/browse/HR-2",
          key: "HR-2",
          issue_type: "Task",
          issue_type_icon:
            "https://halkeye.atlassian.net/rest/api/2/universal_avatar/view/type/issuetype/avatar/10318?size=medium",
          reporter: "Gavin Mogan",
          reporter_email: "atlassian@gavinmogan.com",
          reporter_icon:
            "https://secure.gravatar.com/avatar/43167cef4373ea12339a2309d2a69eba?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FGM-6.png",
          assignee: "",
          assignee_email: "",
          assignee_icon: "",
          status: "Backlog",
          status_category: "To Do",
          status_color: "blue-gray",
          priority: "Medium",
          priority_icon:
            "https://halkeye.atlassian.net/images/icons/priorities/medium_new.svg",
          fields: {},
        }),
      );
    }
    return ogFetch(...arguments);
  };
}

window.docReady = function docReady(fn) {
  if (
    document.readyState === "complete" ||
    document.readyState === "interactive"
  ) {
    setTimeout(fn, 1);
  } else {
    document.addEventListener("DOMContentLoaded", fn);
  }
};

document.addEventListener("click", (event) => {
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return;
  }

  if (target.dataset.confirm && !confirm(target.dataset.confirm)) {
    event.preventDefault();
    event.stopPropagation();
    return;
  }

  if (target.dataset.method && target.href) {
    event.preventDefault();
    event.stopPropagation();
    fetch(target.href, { method: target.dataset.method })
      .then((response) => {
        if (!response.ok) {
          throw new Error(response.statusText);
        }
        if (response.redirected) {
          location.href = response.url;
          return;
        }
        location.reload();
      })
      .catch((error) => alert(error));
  }
});
