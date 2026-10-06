const avatar = (initials: string, background = "#4a154b") =>
  `data:image/svg+xml;utf8,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96"><rect width="96" height="96" fill="${background}"/><text x="48" y="62" font-family="sans-serif" font-size="38" fill="#fff" text-anchor="middle">${initials}</text></svg>`,
  )}`;

const icon = (glyph: string, color: string) =>
  `data:image/svg+xml;utf8,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><circle cx="12" cy="12" r="10" fill="${color}"/><text x="12" y="16" font-family="sans-serif" font-size="12" fill="#fff" text-anchor="middle">${glyph}</text></svg>`,
  )}`;

const profiles: Record<string, unknown> = {
  "ada@example.com": {
    id: "U012AB3CD",
    team_id: "T012AB3CD",
    name: "Ada Lovelace",
    email: "ada@example.com",
    avatar_url: avatar("AL"),
    fields: {
      Title: "Staff Engineer",
      Location: "London, UK",
      Pronouns: "she/her",
    },
  },
  "grace@example.com": {
    id: "U045EF6GH",
    team_id: "T012AB3CD",
    name: "Grace Hopper",
    email: "grace@example.com",
    avatar_url: avatar("GH", "#0052CC"),
    fields: {
      Title: "Principal Engineer",
      Location: "Arlington, VA",
    },
  },
};

const issues: Record<string, unknown> = {
  "HR-1": {
    url: "https://example.atlassian.net/browse/HR-1",
    title: "Add hover cards to the profile page",
    key: "HR-1",
    issue_type: "Task",
    issue_type_icon: icon("T", "#4bade8"),
    reporter: "Grace Hopper",
    reporter_icon: avatar("GH", "#0052CC"),
    assignee: "Ada Lovelace",
    assignee_icon: avatar("AL"),
    status: "In Progress",
    status_color: "#0052CC",
    priority: "High",
    priority_icon: icon("!", "#e5493a"),
    fields: {
      Project: "HR",
      "Due date": "2026-11-01",
    },
  },
  "HR-2": {
    url: "https://example.atlassian.net/browse/HR-2",
    title: "Support multiple Slack workspaces",
    key: "HR-2",
    issue_type: "Story",
    issue_type_icon: icon("S", "#63ba3c"),
    reporter: "Ada Lovelace",
    reporter_icon: avatar("AL"),
    assignee: "Grace Hopper",
    assignee_icon: avatar("GH", "#0052CC"),
    status: "To Do",
    status_color: "#dfe1e6",
    priority: "Medium",
    priority_icon: icon("=", "#ffab00"),
    fields: {
      Project: "HR",
      "Story points": "5",
    },
  },
};

function json(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

export function installFetchMock() {
  const originalFetch = globalThis.fetch.bind(globalThis);

  globalThis.fetch = async (input, init) => {
    const url =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;

    const profileMatch = url.match(/\/profiles\/[^/]+\/([^/?#]+)/);
    if (profileMatch) {
      const email = decodeURIComponent(profileMatch[1]).toLowerCase();
      const data = profiles[email];
      return data ? json(data) : new Response(null, { status: 404 });
    }

    const issueMatch = url.match(/\/issues\/[^/]+\/([^/?#]+)/);
    if (issueMatch) {
      const key = decodeURIComponent(issueMatch[1]).toUpperCase();
      const data = issues[key];
      return data ? json(data) : new Response(null, { status: 404 });
    }

    return originalFetch(input, init);
  };
}
