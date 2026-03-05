import React, { useEffect, useRef, useState } from "react";
import { NavLink, useNavigate, useParams } from "react-router-dom";
import clsx from "clsx";
import { useAuth } from "../auth/AuthProvider";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createWorkspace, listWorkspaces, setWorkspaceSlug } from "../api";
import { Modal } from "../components/Modal";
import { slugify } from "../utils/slug";

export function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { workspace = "" } = useParams();
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const [workspaceModalOpen, setWorkspaceModalOpen] = useState(false);
  const [workspaceName, setWorkspaceName] = useState("");
  const [workspaceSlugInput, setWorkspaceSlugInput] = useState("");
  const [workspaceSlugManuallyEdited, setWorkspaceSlugManuallyEdited] = useState(false);
  const [workspaceDescription, setWorkspaceDescription] = useState("");
  const [workspaceCreateError, setWorkspaceCreateError] = useState<string | null>(null);
  const [workspaceCreating, setWorkspaceCreating] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

  setWorkspaceSlug(workspace);

  const { data: workspaces = [] } = useQuery({
    queryKey: ["workspaces"],
    queryFn: listWorkspaces
  });
  const currentWorkspace = workspaces.find((ws) => ws.slug === workspace);

  useEffect(() => {
    function onDocumentMouseDown(event: MouseEvent) {
      if (!workspaceMenuOpen) {
        return;
      }
      if (!menuRef.current?.contains(event.target as Node)) {
        setWorkspaceMenuOpen(false);
      }
    }
    function onDocumentEscape(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setWorkspaceMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", onDocumentMouseDown);
    document.addEventListener("keydown", onDocumentEscape);
    return () => {
      document.removeEventListener("mousedown", onDocumentMouseDown);
      document.removeEventListener("keydown", onDocumentEscape);
    };
  }, [workspaceMenuOpen]);

  const navItems = [
    { label: "Projects", to: `/${workspace}` },
    { label: "Runs", to: `/${workspace}/runs` },
    { label: "Jobs", to: `/${workspace}/jobs` },
    { label: "Artefacts", to: `/${workspace}/artefacts` },
    { label: "Executors", to: `/${workspace}/executors` },
    { label: "Settings", to: `/${workspace}/settings` }
  ];

  const { user, logoutUrl } = useAuth();
  const displayName = user?.name ?? user?.email ?? "User";
  const avatarInitials = displayName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("") || "U";

  async function onCreateWorkspace() {
    const name = workspaceName.trim();
    const slug = workspaceSlugInput.trim();
    if (!name || !slug) {
      setWorkspaceCreateError("Name and slug are required.");
      return;
    }
    setWorkspaceCreating(true);
    setWorkspaceCreateError(null);
    try {
      const created = await createWorkspace({
        name,
        slug,
        description: workspaceDescription.trim() || undefined
      });
      await queryClient.invalidateQueries({ queryKey: ["workspaces"] });
      setWorkspaceModalOpen(false);
      setWorkspaceMenuOpen(false);
      setWorkspaceName("");
      setWorkspaceSlugInput("");
      setWorkspaceSlugManuallyEdited(false);
      setWorkspaceDescription("");
      navigate(`/${created.slug}`);
    } catch (error) {
      setWorkspaceCreateError((error as Error).message);
    } finally {
      setWorkspaceCreating(false);
    }
  }

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-40 border-b border-ink-100 bg-white/80 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-4 w-48">
            <svg
              viewBox="0 0 650 132"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
            >
              <path
                d="M611.583 93.184C606.548 93.184 601.812 91.9467 597.375 89.472C593.023 86.912 589.524 83.2 586.879 78.336C584.234 73.472 582.911 67.584 582.911 60.672V58.624C582.911 51.712 584.234 45.824 586.879 40.96C589.524 36.096 593.023 32.4267 597.375 29.952C601.727 27.392 606.463 26.112 611.583 26.112C615.423 26.112 618.623 26.5813 621.183 27.52C623.828 28.3733 625.962 29.4827 627.583 30.848C629.204 32.2133 630.442 33.664 631.295 35.2H633.599V1.79199H649.727V91.392H633.855V83.712H631.551C630.1 86.1013 627.839 88.2773 624.767 90.24C621.78 92.2027 617.386 93.184 611.583 93.184ZM616.447 79.104C621.396 79.104 625.535 77.5253 628.863 74.368C632.191 71.1253 633.855 66.432 633.855 60.288V59.008C633.855 52.864 632.191 48.2133 628.863 45.056C625.62 41.8133 621.482 40.192 616.447 40.192C611.498 40.192 607.359 41.8133 604.031 45.056C600.703 48.2133 599.039 52.864 599.039 59.008V60.288C599.039 66.432 600.703 71.1253 604.031 74.368C607.359 77.5253 611.498 79.104 616.447 79.104Z"
                fill="#223751"
              />
              <path
                d="M537.483 91.392V27.9039H553.355V35.0719H555.659C556.598 32.5119 558.134 30.6346 560.267 29.4399C562.486 28.2453 565.046 27.6479 567.947 27.6479H575.627V41.9839H567.691C563.595 41.9839 560.224 43.0933 557.579 45.312C554.934 47.4453 553.611 50.7733 553.611 55.296V91.392H537.483Z"
                fill="#223751"
              />
              <path
                d="M483.195 93.184C478.672 93.184 474.619 92.416 471.035 90.88C467.451 89.2587 464.592 86.9547 462.459 83.968C460.411 80.896 459.387 77.184 459.387 72.832C459.387 68.48 460.411 64.8533 462.459 61.952C464.592 58.9653 467.494 56.7467 471.163 55.296C474.918 53.76 479.184 52.992 483.963 52.992H501.371V49.408C501.371 46.4213 500.432 43.9893 498.555 42.112C496.678 40.1493 493.691 39.168 489.595 39.168C485.584 39.168 482.598 40.1067 480.635 41.984C478.672 43.776 477.392 46.1227 476.795 49.024L461.947 44.032C462.971 40.7893 464.592 37.8453 466.811 35.2C469.115 32.4693 472.144 30.2933 475.899 28.672C479.739 26.9653 484.39 26.112 489.851 26.112C498.214 26.112 504.827 28.2027 509.691 32.384C514.555 36.5653 516.987 42.624 516.987 50.56V74.24C516.987 76.8 518.182 78.08 520.571 78.08H525.691V91.392H514.939C511.782 91.392 509.179 90.624 507.131 89.088C505.083 87.552 504.059 85.504 504.059 82.944V82.816H501.627C501.286 83.84 500.518 85.2053 499.323 86.912C498.128 88.5333 496.251 89.984 493.691 91.264C491.131 92.544 487.632 93.184 483.195 93.184ZM486.011 80.128C490.534 80.128 494.203 78.8907 497.019 76.416C499.92 73.856 501.371 70.4853 501.371 66.304V65.024H485.115C482.128 65.024 479.782 65.664 478.075 66.944C476.368 68.224 475.515 70.016 475.515 72.32C475.515 74.624 476.411 76.5013 478.203 77.952C479.995 79.4027 482.598 80.128 486.011 80.128Z"
                fill="#223751"
              />
              <path
                d="M346.648 117.4V131.48H433.584C437.851 131.48 441.264 130.157 443.824 127.512C446.384 124.952 447.664 121.496 447.664 117.144V28.392H431.536V74.648C431.536 80.536 430.171 85.144 427.44 88.472C424.709 91.7146 420.869 93.336 415.92 93.336H388.92C389.041 93.5944 389.166 93.8504 389.296 94.104C391.344 98.1146 394.203 101.272 397.872 103.576C401.627 105.795 405.979 106.904 410.928 106.904C414.768 106.904 417.925 106.435 420.4 105.496C422.875 104.557 424.837 103.363 426.288 101.912C427.739 100.461 428.805 99.0106 429.488 97.56H431.792V113.56C431.792 116.12 430.597 117.4 428.208 117.4H346.648Z"
                fill="#223751"
              />
              <path
                d="M347.733 91.392V1.79199H363.861V35.712H366.165C366.848 34.3467 367.914 32.9813 369.365 31.616C370.816 30.2507 372.736 29.1413 375.125 28.288C377.6 27.3493 380.714 26.88 384.469 26.88C389.418 26.88 393.728 28.032 397.397 30.336C401.152 32.5547 404.053 35.6693 406.101 39.68C407.273 41.9746 406.971 44.6352 406.101 44.6352C403.648 44.6352 399.909 44.6352 397.397 44.6352C393.148 44.6352 391.648 44.6352 389.461 44.032C387.157 41.6427 383.829 40.448 379.477 40.448C374.528 40.448 370.688 42.112 367.957 45.44C365.226 48.6827 363.861 53.248 363.861 59.136V91.392H347.733Z"
                fill="#223751"
              />
              <path
                d="M302.557 93.184C296.413 93.184 290.824 91.904 285.789 89.344C280.84 86.784 276.914 83.072 274.013 78.208C271.112 73.344 269.661 67.456 269.661 60.544V58.752C269.661 51.84 271.112 45.952 274.013 41.088C276.914 36.224 280.84 32.512 285.789 29.952C290.824 27.392 296.413 26.112 302.557 26.112C308.616 26.112 313.821 27.1787 318.173 29.312C322.525 31.4453 326.024 34.3893 328.669 38.144C331.4 41.8133 333.192 45.9947 334.045 50.688L318.429 54.016C318.088 51.456 317.32 49.152 316.125 47.104C314.93 45.056 313.224 43.4347 311.005 42.24C308.872 41.0453 306.184 40.448 302.941 40.448C299.698 40.448 296.754 41.1733 294.109 42.624C291.549 43.9893 289.501 46.08 287.965 48.896C286.514 51.6267 285.789 54.9973 285.789 59.008V60.288C285.789 64.2987 286.514 67.712 287.965 70.528C289.501 73.2587 291.549 75.3493 294.109 76.8C296.754 78.1653 299.698 78.848 302.941 78.848C307.805 78.848 311.474 77.6107 313.949 75.136C316.509 72.576 318.13 69.248 318.813 65.152L334.429 68.864C333.32 73.3867 331.4 77.5253 328.669 81.28C326.024 84.9493 322.525 87.8507 318.173 89.984C313.821 92.1173 308.616 93.184 302.557 93.184Z"
                fill="#223751"
              />
              <path
                d="M240.928 91.392C236.747 91.392 233.333 90.112 230.688 87.552C228.128 84.9067 226.848 81.408 226.848 77.056V41.216H210.976V27.904H226.848V8.19202H242.976V27.904H260.384V41.216H242.976V74.24C242.976 76.8 244.171 78.08 246.56 78.08H258.848V91.392H240.928Z"
                fill="#223751"
              />
              <path
                d="M182.608 91.392V27.904H198.736V91.392H182.608ZM190.672 20.48C187.771 20.48 185.296 19.5413 183.248 17.664C181.285 15.7867 180.304 13.312 180.304 10.24C180.304 7.168 181.285 4.69334 183.248 2.816C185.296 0.938667 187.771 0 190.672 0C193.659 0 196.133 0.938667 198.096 2.816C200.059 4.69334 201.04 7.168 201.04 10.24C201.04 13.312 200.059 15.7867 198.096 17.664C196.133 19.5413 193.659 20.48 190.672 20.48Z"
                fill="#223751"
              />
              <path
                d="M87.353 91.392L78.393 27.904H94.393L100.025 80.512H102.329L110.521 27.904H136.377L144.569 80.512H146.873L152.505 27.904H168.505L159.545 91.392H132.793L124.601 38.784H122.297L114.105 91.392H87.353Z"
                fill="#223751"
              />
              <path
                d="M35.2 93.184C28.288 93.184 22.1867 91.9467 16.896 89.472C11.6053 86.9973 7.46667 83.456 4.48 78.848C1.49333 74.24 0 68.6933 0 62.208V58.624H16.64V62.208C16.64 67.584 18.304 71.6373 21.632 74.368C24.96 77.0133 29.4827 78.336 35.2 78.336C41.0027 78.336 45.312 77.184 48.128 74.88C51.0293 72.576 52.48 69.632 52.48 66.048C52.48 63.5733 51.7547 61.568 50.304 60.032C48.9387 58.496 46.8907 57.2587 44.16 56.32C41.5147 55.296 38.272 54.3573 34.432 53.504L31.488 52.864C25.344 51.4987 20.0533 49.792 15.616 47.744C11.264 45.6107 7.89333 42.8373 5.504 39.424C3.2 36.0107 2.048 31.5733 2.048 26.112C2.048 20.6507 3.328 16 5.888 12.16C8.53333 8.23467 12.2027 5.248 16.896 3.2C21.6747 1.06667 27.264 0 33.664 0C40.064 0 45.7387 1.10933 50.688 3.328C55.7227 5.46133 59.648 8.704 62.464 13.056C65.3653 17.3227 66.816 22.6987 66.816 29.184V33.024H50.176V29.184C50.176 25.7707 49.4933 23.04 48.128 20.992C46.848 18.8587 44.9707 17.3227 42.496 16.384C40.0213 15.36 37.0773 14.848 33.664 14.848C28.544 14.848 24.7467 15.8293 22.272 17.792C19.8827 19.6693 18.688 22.272 18.688 25.6C18.688 27.8187 19.2427 29.696 20.352 31.232C21.5467 32.768 23.296 34.048 25.6 35.072C27.904 36.096 30.848 36.992 34.432 37.76L37.376 38.4C43.776 39.7653 49.3227 41.5147 54.016 43.648C58.7947 45.7813 62.5067 48.5973 65.152 52.096C67.7973 55.5947 69.12 60.0747 69.12 65.536C69.12 70.9973 67.712 75.8187 64.896 80C62.1653 84.096 58.24 87.3387 53.12 89.728C48.0853 92.032 42.112 93.184 35.2 93.184Z"
                fill="#223751"
              />
              <path
                d="M373.67 88.0739C373.78 90.5796 379.331 90.298 383.07 90.3099C391.57 90.3338 400.075 90.2622 408.575 90.1763L414.946 90.1143C415.425 90.1143 416.129 90.0952 416.777 90.0284C417.103 89.9806 417.426 89.9377 417.752 89.8828L418.714 89.6513C419.975 89.2838 421.177 88.711 422.249 87.9594C424.394 86.4631 426.055 84.196 426.704 81.5875C427.365 78.9911 427.034 76.2349 425.821 73.8723C425.514 73.2542 425.098 72.6576 424.823 72.24L423.896 70.8583L416.5 59.7927L415.569 58.4063C415.207 57.886 414.839 57.3801 414.429 56.898C412.806 54.9746 410.777 53.3662 408.498 52.2732C406.224 51.1683 403.718 50.5455 401.192 50.4715C400.524 50.4477 400.022 50.4596 399.452 50.4524H397.786H394.452L381.116 50.4644C378.193 50.4644 373.849 49.8654 373.665 52.1133C373.464 54.5451 378.009 53.8602 380.786 53.8912L393.752 53.97C393.666 54.4353 393.704 54.9341 393.704 55.3565C393.693 56.6523 393.704 57.9051 393.736 59.2415C393.772 60.8428 394.152 62.4417 394.82 63.8902C396.156 66.804 398.65 69.0758 401.584 70.2309C402.35 70.5292 403.148 70.7631 403.961 70.8967C404.373 70.9587 404.775 71.0375 405.187 71.0423C405.591 71.0613 406.039 71.09 406.345 71.078H410.391L418.479 71.0661C418.895 71.0661 419.398 71.195 419.827 71.1019L421.971 74.3021C422.297 74.7841 422.51 75.1134 422.718 75.5478C422.919 75.963 423.087 76.3902 423.209 76.8365C423.453 77.7218 423.534 78.6501 423.448 79.5737C423.283 81.3873 422.369 83.1055 420.978 84.3273C419.592 85.5563 417.773 86.277 415.935 86.3438C415.511 86.3677 414.91 86.3438 414.383 86.3438L412.753 86.32L409.493 86.2651C400.804 86.1052 392.119 85.8498 383.442 85.4346C379.392 85.2389 373.521 85.0074 373.65 88.062L373.67 88.0739ZM410.681 67.5654L406.722 67.5845C406.023 67.5845 405.489 67.5845 404.919 67.5106C404.354 67.4366 403.799 67.3101 403.258 67.1192C401.106 66.3794 399.306 64.6565 398.521 62.5803C398.123 61.547 397.975 60.4349 398.08 59.3658C398.092 59.1272 398.135 58.7549 398.171 58.4375L398.293 57.4543C398.38 56.8005 398.47 56.1418 398.576 55.4879C398.65 55.0226 398.729 54.4809 398.631 53.9917H400.335C402.363 53.9797 404.33 54.3401 406.169 55.1156C408.014 55.8673 409.673 57.0223 411.059 58.4446C411.77 59.1653 412.328 59.929 412.921 60.8143L414.736 63.5157L417.493 67.6394C415.219 67.5773 412.94 67.5654 410.666 67.5726L410.681 67.5654Z"
                fill="#60A5FA"
              />
              <path
                d="M378.242 64.2915C378.194 66.2651 378.541 68.2386 378.541 70.2169C378.541 72.1953 378.529 74.3478 378.553 76.4122C378.553 76.7535 377.952 77.2546 380.207 77.2809C382.646 77.3048 381.959 76.775 381.99 76.4552C382.344 72.4174 381.444 68.3795 383.117 64.3468C383.314 63.8767 383.546 63.1918 380.482 63.2109C377.969 63.2228 378.251 63.8695 378.239 64.3039L378.242 64.2915Z"
                fill="#60A5FA"
              />
            </svg>
          </div>
          <div className="flex items-center gap-4">
            {/* <button
              type="button"
              className="hidden w-72 rounded-full border border-ink-200 bg-white px-4 py-2 text-left text-sm text-ink-400 shadow-sm md:block"
            >
              ⌘K Search
            </button> */}
            <div className="relative" ref={menuRef}>
              <button
                type="button"
                className="rounded-full border border-ink-200 bg-white px-3 py-1 text-xs text-ink-700"
                onClick={() => setWorkspaceMenuOpen((open) => !open)}
              >
                {currentWorkspace?.name ?? workspace} ▾
              </button>
              {workspaceMenuOpen && (
                <div className="absolute right-0 top-10 z-50 w-72 rounded-xl border border-ink-200 bg-white p-3 shadow-lg">
                  <label className="text-[11px] uppercase tracking-[0.15em] text-ink-500">Workspace</label>
                  <select
                    className="mt-2 w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-700"
                    value={workspace}
                    onChange={(e) => {
                      navigate(`/${e.target.value}`);
                      setWorkspaceMenuOpen(false);
                    }}
                  >
                    {workspaces.map((ws) => (
                      <option key={ws.slug} value={ws.slug}>
                        {ws.name}
                      </option>
                    ))}
                  </select>
                  <button
                    type="button"
                    className="mt-3 w-full rounded-full border border-ink-200 px-3 py-2 text-sm font-medium text-ink-700 hover:bg-ink-50"
                    onClick={() => {
                      setWorkspaceModalOpen(true);
                      setWorkspaceMenuOpen(false);
                      setWorkspaceSlugManuallyEdited(false);
                    }}
                  >
                    Create Workspace
                  </button>
                </div>
              )}
            </div>
            <div className="hidden text-right md:block">
              <p className="text-xs font-semibold text-ink-900">{displayName}</p>
              {user?.email && <p className="text-[11px] text-ink-500">{user.email}</p>}
            </div>
            {user?.picture_url ? (
              <img
                src={user.picture_url}
                alt={displayName}
                className="h-9 w-9 rounded-full border border-ink-200 object-cover"
              />
            ) : (
              <div className="flex h-9 w-9 items-center justify-center rounded-full bg-ink-900 text-xs font-semibold text-white">
                {avatarInitials}
              </div>
            )}
            <button
              type="button"
              className="rounded-full border border-ink-200 px-3 py-1 text-xs text-ink-600"
              onClick={() => window.location.assign(logoutUrl)}
            >
              Logout
            </button>
          </div>
        </div>
      </header>
      <div className="mx-auto flex max-w-6xl gap-6 px-6 py-8">
        <aside className="hidden w-56 flex-shrink-0 md:block">
          <nav className="surface p-4">
            <p className="text-xs uppercase tracking-[0.2em] text-ink-400">Navigation</p>
            <div className="mt-4 flex flex-col gap-1">
              {navItems.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === `/${workspace}`}
                  className={({ isActive }) =>
                    clsx(
                      "rounded-lg px-3 py-2 text-sm font-medium",
                      isActive
                        ? "bg-ink-900 text-white"
                        : "text-ink-500 hover:bg-ink-100 hover:text-ink-900"
                    )
                  }
                >
                  {item.label}
                </NavLink>
              ))}
            </div>
          </nav>
        </aside>
        <main className="flex-1 min-w-0 space-y-8">{children}</main>
      </div>
      <Modal
        open={workspaceModalOpen}
        title="Create Workspace"
        description="Create a new workspace for projects, members, and runs."
        onClose={() => setWorkspaceModalOpen(false)}
        footer={
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setWorkspaceModalOpen(false)}
              className="text-sm text-ink-500"
            >
              Close
            </button>
            <button
              type="button"
              onClick={onCreateWorkspace}
              disabled={workspaceCreating}
              className="rounded-full bg-ink-900 px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
            >
              Create
            </button>
          </div>
        }
      >
        <div className="space-y-4 text-sm text-ink-600">
          {workspaceCreateError && <p className="text-sm text-red-600">{workspaceCreateError}</p>}
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Name</label>
            <input
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="Acme"
              value={workspaceName}
              onChange={(e) => {
                const nextName = e.target.value;
                setWorkspaceName(nextName);
                if (!workspaceSlugManuallyEdited) {
                  setWorkspaceSlugInput(slugify(nextName));
                }
              }}
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Slug</label>
            <input
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="acme"
              value={workspaceSlugInput}
              onChange={(e) => {
                const nextSlug = slugify(e.target.value);
                setWorkspaceSlugInput(nextSlug);
                setWorkspaceSlugManuallyEdited(nextSlug.trim().length > 0);
              }}
            />
          </div>
          <div>
            <label className="text-xs uppercase tracking-[0.2em] text-ink-400">Description</label>
            <textarea
              className="mt-2 w-full rounded-lg border border-ink-200 px-3 py-2"
              placeholder="Optional description"
              rows={3}
              value={workspaceDescription}
              onChange={(e) => setWorkspaceDescription(e.target.value)}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
