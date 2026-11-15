%global debug_package %{nil}
%global goipath github.com/ptitlu42/highrise-code

Name:           highrise-code
Version:        0.1.0
Release:        1%{?dist}
Summary:        Interactive treemap city visualization of a project
License:        MIT
URL:            https://%{goipath}
Source0:        %{url}/archive/refs/tags/v%{version}.tar.gz#/%{name}-%{version}.tar.gz
BuildRequires:  golang >= 1.24
BuildRequires:  make

%description
highrise-code scans a project directory and renders its architecture as an
interactive city where directories are districts and files are buildings.
It reports detailed line and character statistics and can write a standalone
HTML file. It is a single static binary with no network access.

%prep
%autosetup -n %{name}-%{version}

%build
export CGO_ENABLED=0
export GOFLAGS="-trimpath -buildmode=pie -mod=readonly"
go build -ldflags "-s -w -X %{goipath}/internal/cli.Version=%{version}" -o bin/%{name} ./cmd/%{name}

%check
go test ./...

%install
make install DESTDIR=%{buildroot} PREFIX=%{_prefix} VERSION=%{version} GO=true

%files
%license LICENSE
%{_bindir}/%{name}
%{_mandir}/man1/%{name}.1*
%{_datadir}/bash-completion/completions/%{name}
%{_datadir}/zsh/site-functions/_%{name}
%{_datadir}/fish/vendor_completions.d/%{name}.fish
%{_datadir}/applications/%{name}.desktop
%{_datadir}/icons/hicolor/scalable/apps/%{name}.svg

%changelog
* Sat Nov 14 2026 ptitlu <ptitlu42@users.noreply.github.com> - 0.1.0-1
- Initial release
