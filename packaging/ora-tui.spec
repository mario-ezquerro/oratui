Name:           ora-tui
Version:        1.0.0
Release:        1%{?dist}
Summary:        Oracle Cluster (RAC/Restart) & Single Instance TUI Manager
License:        Apache-2.0
URL:            https://github.com/mario/ora-tui
BuildArch:      x86_64

%description
Zero-dependency static Go Terminal User Interface (TUI) for Oracle Linux / RHEL 7, 8, and 9.
Automatically discovers Oracle RAC, Oracle Restart, and Single Instance environments,
monitors connected users, tablespaces, tables, ASM, RMAN, Data Guard, and Alert Logs,
and executes safe operational workflows via arrow-key Unicode box-drawing menus.

%install
mkdir -p %{buildroot}%{_bindir}
mkdir -p %{buildroot}%{_sysconfdir}/sudoers.d
install -m 0755 bin/ora-tui-linux-amd64 %{buildroot}%{_bindir}/ora-tui
install -m 0440 packaging/sudoers.ora-tui %{buildroot}%{_sysconfdir}/sudoers.d/ora-tui

%files
%{_bindir}/ora-tui
%config(noreplace) %attr(0440,root,root) %{_sysconfdir}/sudoers.d/ora-tui

%changelog
* Thu Oct 15 2026 Mario <mario@ora-tui.local> - 1.0.0-1
- Initial release of ora-tui with RAC/Single Instance discovery, 8 monitoring views, and Unicode box-drawing menus.
