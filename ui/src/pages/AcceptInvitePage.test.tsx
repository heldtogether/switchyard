import React from "react";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { vi } from "vitest";
import { AcceptInvitePage } from "./AcceptInvitePage";
import { renderWithProviders } from "../test/render";

const navigateMock = vi.fn();
const acceptWorkspaceInviteMock = vi.fn();
const acceptProjectInviteMock = vi.fn();
const listWorkspacesMock = vi.fn();
const useAuthMock = vi.fn();

vi.mock("../api", () => ({
  acceptWorkspaceInvite: (...args: any[]) => acceptWorkspaceInviteMock(...args),
  acceptProjectInvite: (...args: any[]) => acceptProjectInviteMock(...args),
  listWorkspaces: (...args: any[]) => listWorkspacesMock(...args)
}));

vi.mock("../auth/AuthProvider", () => ({
  useAuth: () => useAuthMock()
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useLocation: () => ({ pathname: "/accept-invite", search: "?token=tok-123" }),
    useNavigate: () => navigateMock,
    Navigate: ({ to }: { to: string }) => <div>redirect:{to}</div>
  };
});

describe("AcceptInvitePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useAuthMock.mockReset();
    acceptWorkspaceInviteMock.mockReset();
    acceptProjectInviteMock.mockReset();
    listWorkspacesMock.mockReset();
  });

  it("redirects to login when unauthenticated", () => {
    useAuthMock.mockReturnValue({
      isLoading: false,
      isAuthenticated: false
    });

    renderWithProviders(<AcceptInvitePage />);

    expect(screen.getByText("redirect:/login?next=%2Faccept-invite%3Ftoken%3Dtok-123")).toBeInTheDocument();
  });

  it("falls back to project invite acceptance when workspace acceptance fails", async () => {
    useAuthMock.mockReturnValue({
      isLoading: false,
      isAuthenticated: true
    });
    acceptWorkspaceInviteMock.mockRejectedValue(new Error("workspace invite not found"));
    acceptProjectInviteMock.mockResolvedValue({ message: "Invite accepted" });
    listWorkspacesMock.mockResolvedValue([
      {
        id: "w1",
        slug: "team",
        name: "Team",
        description: "",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString()
      }
    ]);

    renderWithProviders(<AcceptInvitePage />);

    await waitFor(() => {
      expect(acceptWorkspaceInviteMock).toHaveBeenCalledWith("tok-123");
      expect(acceptProjectInviteMock).toHaveBeenCalledWith("tok-123");
      expect(navigateMock).toHaveBeenCalledWith("/team?invite=accepted", { replace: true });
    });
  });

  it("shows retry when no workspace is visible after acceptance", async () => {
    useAuthMock.mockReturnValue({
      isLoading: false,
      isAuthenticated: true
    });
    acceptWorkspaceInviteMock.mockResolvedValue({ message: "Invite accepted" });
    acceptProjectInviteMock.mockResolvedValue({ message: "Invite accepted" });
    listWorkspacesMock.mockResolvedValue([]);

    renderWithProviders(<AcceptInvitePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Invite accepted, but no workspace membership is visible yet. Retry in a moment.")
      ).toBeInTheDocument();
    });
    expect(navigateMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => {
      expect(acceptWorkspaceInviteMock).toHaveBeenCalledTimes(2);
    });
  });
});
