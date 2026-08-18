Unicode true

####
## Installeur Windows de Comptoir.
##
## Ce fichier est repris de la trame produite par Wails, puis adapté :
## interface en français, affichage de la licence — dont l'article 3 sur la
## paternité doit être lu avant installation — et désinstallation qui ne touche
## jamais aux données du commerçant.
##
## Pour le reconstruire :
##     wails build -platform windows/amd64 -nsis
####

!include "wails_tools.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "Installation de ${INFO_PRODUCTNAME}"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# L'interface est rendue par WebView2 : sans cette déclaration, Windows
# l'étirerait sur les écrans à forte densité.
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING

# --- Accueil ----------------------------------------------------------------
!define MUI_WELCOMEPAGE_TITLE "Installation de Comptoir"
!define MUI_WELCOMEPAGE_TEXT "Comptoir tient le stock, les ventes, les achats et les comptes de votre boutique.$\r$\n$\r$\nIl fonctionne entièrement sur ce poste : aucune donnée ne part sur internet, il n'y a ni compte à créer ni abonnement.$\r$\n$\r$\nÀ la fin de l'installation, un assistant en sept étapes vous guidera pour configurer votre entreprise, votre monnaie et votre premier compte.$\r$\n$\r$\nCliquez sur Suivant pour continuer."

# --- Licence ----------------------------------------------------------------
# L'article 3 protège la paternité de l'éditeur : elle doit être lue, pas
# découverte après coup.
!define MUI_LICENSEPAGE_TEXT_TOP "Veuillez lire les conditions d'utilisation."
!define MUI_LICENSEPAGE_TEXT_BOTTOM "Le logiciel est libre d'usage, y compris commercial. En contrepartie, l'article 3 impose le maintien de la mention de son éditeur. Acceptez pour poursuivre."
!define MUI_LICENSEPAGE_BUTTON "J'accepte"

# --- Fin --------------------------------------------------------------------
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_FINISHPAGE_TITLE "Comptoir est installé"
!define MUI_FINISHPAGE_TEXT "L'assistant de configuration s'ouvrira au premier lancement.$\r$\n$\r$\nVos données seront enregistrées dans %APPDATA%\Comptoir. Elles ne seront jamais supprimées par une désinstallation ni par une mise à jour."
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "Ouvrir Comptoir maintenant"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\..\..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

# --- Désinstallation --------------------------------------------------------
!define MUI_UNCONFIRMPAGE_TEXT_TOP "Comptoir va être retiré de ce poste. Vos données de gestion — articles, ventes, clients, sauvegardes — ne seront pas supprimées : elles restent dans %APPDATA%\Comptoir."
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "French"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"

!ifdef WAILS_INSTALL_SCOPE
  !if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
  !else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
  !endif
!else
  InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif

ShowInstDetails show

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Section "Comptoir" SectionPrincipale
    !insertmacro wails.setShellContext

    # WebView2 rend l'interface : sans lui, l'application ne s'ouvre pas.
    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    # Cache de WebView2 propre à l'application. Ce chemin est celui de
    # l'exécutable (« Comptoir.exe »), distinct du dossier de données
    # « %APPDATA%\Comptoir » — que la désinstallation ne doit jamais toucher.
    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}"

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
